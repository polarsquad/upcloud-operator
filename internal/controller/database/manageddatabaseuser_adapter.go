package database

import (
	"context"
	"fmt"
	"reflect"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	databasev1alpha1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/k8s"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/resolve"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// ManagedDatabaseUserAdapter maps ManagedDatabaseUser onto the UpCloud
// Managed Database user API.
type ManagedDatabaseUserAdapter struct {
	API    upcloudapi.DatabaseAPI
	Client client.Client
}

var _ reconciler.Adapter[*databasev1alpha1.ManagedDatabaseUser] = (*ManagedDatabaseUserAdapter)(nil)

// parentUUID returns the service UUID, resolving the referenced
// ManagedDatabase (which must be Ready) on first use and storing the result
// in Status.
func (a *ManagedDatabaseUserAdapter) parentUUID(ctx context.Context, u *databasev1alpha1.ManagedDatabaseUser) (string, error) {
	if u.Status.ServiceUUID != "" {
		return u.Status.ServiceUUID, nil
	}
	uuid, err := resolve.Ready(ctx, a.Client, u.Namespace, u.Spec.ServiceRef.Name, &databasev1alpha1.ManagedDatabase{})
	if err != nil {
		return "", err
	}
	u.Status.ServiceUUID = uuid
	return uuid, nil
}

// passwordFromSecret reads one key from a Secret, reporting whether the
// source existed at all.
func (a *ManagedDatabaseUserAdapter) passwordFromSecret(ctx context.Context, namespace, name, key string) (string, bool, error) {
	var sec corev1.Secret
	err := a.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &sec)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("get secret %q: %w", name, err)
	}
	val, ok := sec.Data[key]
	if !ok {
		return "", false, fmt.Errorf("secret %q has no key %q", name, key)
	}
	return string(val), true, nil
}

// desiredPassword returns the password the user should have. With
// passwordSecretRef that Secret is the source of truth; otherwise the
// credentials Secret is. The second return value reports whether a source
// was found at all.
func (a *ManagedDatabaseUserAdapter) desiredPassword(ctx context.Context, u *databasev1alpha1.ManagedDatabaseUser) (string, bool, error) {
	if u.Spec.PasswordSecretRef != nil {
		return a.passwordFromSecret(ctx, u.Namespace, u.Spec.PasswordSecretRef.Name, u.Spec.PasswordSecretRef.Key)
	}
	return a.passwordFromSecret(ctx, u.Namespace, u.ConnectionSecretName(), SecretKeyPassword)
}

// Observe implements reconciler.Adapter. A missing user reports
// Exists: false. A missing credentials Secret (generated password) is
// healed from the API by rewriting the Secret.
func (a *ManagedDatabaseUserAdapter) Observe(ctx context.Context, u *databasev1alpha1.ManagedDatabaseUser) (reconciler.Observation, error) {
	svcUUID, err := a.parentUUID(ctx, u)
	if err != nil {
		return reconciler.Observation{}, err
	}
	user, err := a.API.GetManagedDatabaseUser(ctx, &request.GetManagedDatabaseUserRequest{ServiceUUID: svcUUID, Username: u.ExternalUsername()})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get managed database user: %w", err)
	}
	u.Status.Username = user.Username
	u.Status.Type = string(user.Type)

	desired, found, err := a.desiredPassword(ctx, u)
	if err != nil {
		return reconciler.Observation{}, err
	}
	if !found {
		if u.Spec.PasswordSecretRef != nil {
			return reconciler.Observation{}, fmt.Errorf("password secret %q: %w", u.Spec.PasswordSecretRef.Name, reconciler.ErrDependencyNotReady)
		}
		// Credentials Secret was deleted: heal it from the API password.
		svc, err := a.API.GetManagedDatabase(ctx, &request.GetManagedDatabaseRequest{UUID: svcUUID})
		if err != nil {
			return reconciler.Observation{}, fmt.Errorf("get managed database: %w", err)
		}
		if err := a.writeCredentialsSecret(ctx, u, svc, user.Password); err != nil {
			return reconciler.Observation{}, err
		}
	} else if user.Password != desired {
		return reconciler.Observation{Exists: true, UpToDate: false, Ready: true, Message: "password out of date"}, nil
	}
	if !accessControlMatches(u, user) {
		return reconciler.Observation{Exists: true, UpToDate: false, Ready: true, Message: "access control out of date"}, nil
	}
	return reconciler.Observation{Exists: true, UpToDate: true, Ready: true, Message: "user " + user.Username}, nil
}

// Create implements reconciler.Adapter. A Conflict (user already exists in
// UpCloud) is treated as adoption: the next Observe reconciles the details.
func (a *ManagedDatabaseUserAdapter) Create(ctx context.Context, u *databasev1alpha1.ManagedDatabaseUser) error {
	svcUUID, err := a.parentUUID(ctx, u)
	if err != nil {
		return err
	}
	svc, err := a.API.GetManagedDatabase(ctx, &request.GetManagedDatabaseRequest{UUID: svcUUID})
	if err != nil {
		return fmt.Errorf("get managed database: %w", err)
	}

	password := ""
	req := &request.CreateManagedDatabaseUserRequest{
		ServiceUUID:             svcUUID,
		Username:                u.ExternalUsername(),
		PGAccessControl:         pgRequest(u.Spec.PGAccessControl),
		ValkeyAccessControl:     valkeyRequest(u.Spec.ValkeyAccessControl),
		OpenSearchAccessControl: openSearchRequest(u.Spec.OpenSearchAccessControl),
	}
	if u.Spec.PasswordSecretRef != nil {
		pw, found, err := a.desiredPassword(ctx, u)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("password secret %q: %w", u.Spec.PasswordSecretRef.Name, reconciler.ErrDependencyNotReady)
		}
		password = pw
		req.Password = pw
	}
	if u.Spec.Authentication != "" {
		req.Authentication = upcloud.ManagedDatabaseUserAuthenticationType(u.Spec.Authentication)
	}
	created, err := a.API.CreateManagedDatabaseUser(ctx, req)
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles it
		}
		return fmt.Errorf("create managed database user: %w", err)
	}
	u.Status.Username = created.Username
	u.Status.Type = string(created.Type)

	if u.Spec.PasswordSecretRef == nil {
		password = created.Password
	}
	return a.writeCredentialsSecret(ctx, u, svc, password)
}

// Update implements reconciler.Adapter. Password drift is pushed with
// ModifyManagedDatabaseUser, access control drift with
// ModifyManagedDatabaseUserAccessControl.
func (a *ManagedDatabaseUserAdapter) Update(ctx context.Context, u *databasev1alpha1.ManagedDatabaseUser) error {
	svcUUID, err := a.parentUUID(ctx, u)
	if err != nil {
		return err
	}
	user, err := a.API.GetManagedDatabaseUser(ctx, &request.GetManagedDatabaseUserRequest{ServiceUUID: svcUUID, Username: u.ExternalUsername()})
	if upcloudapi.IsNotFound(err) {
		return reconciler.ErrPending
	}
	if err != nil {
		return fmt.Errorf("get managed database user: %w", err)
	}

	desired, found, err := a.desiredPassword(ctx, u)
	if err != nil {
		return err
	}
	if !found {
		if u.Spec.PasswordSecretRef != nil {
			return fmt.Errorf("password secret %q: %w", u.Spec.PasswordSecretRef.Name, reconciler.ErrDependencyNotReady)
		}
		return nil // missing credentials Secret is healed in Observe
	}
	if user.Password != desired {
		modify := &request.ModifyManagedDatabaseUserRequest{
			ServiceUUID: svcUUID,
			Username:    user.Username,
			Password:    desired,
		}
		if u.Spec.Authentication != "" {
			modify.Authentication = upcloud.ManagedDatabaseUserAuthenticationType(u.Spec.Authentication)
		}
		if _, err := a.API.ModifyManagedDatabaseUser(ctx, modify); err != nil {
			return fmt.Errorf("modify managed database user: %w", err)
		}
	}
	if !accessControlMatches(u, user) {
		if _, err := a.API.ModifyManagedDatabaseUserAccessControl(ctx, &request.ModifyManagedDatabaseUserAccessControlRequest{
			ServiceUUID:             svcUUID,
			Username:                user.Username,
			PGAccessControl:         pgRequest(u.Spec.PGAccessControl),
			ValkeyAccessControl:     valkeyRequest(u.Spec.ValkeyAccessControl),
			OpenSearchAccessControl: openSearchRequest(u.Spec.OpenSearchAccessControl),
		}); err != nil {
			return fmt.Errorf("modify managed database user access control: %w", err)
		}
	}
	return nil
}

// Delete implements reconciler.Adapter. Deleting the primary user is
// refused; a gone service (404) counts as deleted.
func (a *ManagedDatabaseUserAdapter) Delete(ctx context.Context, u *databasev1alpha1.ManagedDatabaseUser) error {
	if u.Status.ServiceUUID == "" || u.Status.Username == "" {
		return nil
	}
	if u.Status.Type == string(upcloud.ManagedDatabaseUserTypePrimary) {
		return fmt.Errorf("cannot delete primary user %q", u.ExternalUsername())
	}
	err := a.API.DeleteManagedDatabaseUser(ctx, &request.DeleteManagedDatabaseUserRequest{
		ServiceUUID: u.Status.ServiceUUID,
		Username:    u.ExternalUsername(),
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return reconciler.ErrPending
	default:
		return fmt.Errorf("delete managed database user: %w", err)
	}
}

// writeCredentialsSecret stores the user's credentials in an owned Secret.
func (a *ManagedDatabaseUserAdapter) writeCredentialsSecret(ctx context.Context, u *databasev1alpha1.ManagedDatabaseUser, svc *upcloud.ManagedDatabase, password string) error {
	p := svc.ServiceURIParams
	uri := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		u.ExternalUsername(), password, p.Host, p.Port, p.DatabaseName, p.SSLMode)
	data := map[string][]byte{
		SecretKeyUsername: []byte(u.ExternalUsername()),
		SecretKeyPassword: []byte(password),
		SecretKeyHost:     []byte(p.Host),
		SecretKeyPort:     []byte(p.Port),
		SecretKeyURI:      []byte(uri),
	}
	return k8s.WriteOwnedSecret(ctx, a.Client, u, u.ConnectionSecretName(), data)
}

// accessControlMatches compares the spec access control blocks against the
// observed user.
func accessControlMatches(u *databasev1alpha1.ManagedDatabaseUser, user *upcloud.ManagedDatabaseUser) bool {
	if !pgMatches(u.Spec.PGAccessControl, user.PGAccessControl) {
		return false
	}
	if !valkeyMatches(u.Spec.ValkeyAccessControl, user.ValkeyAccessControl) {
		return false
	}
	return openSearchMatches(u.Spec.OpenSearchAccessControl, user.OpenSearchAccessControl)
}

func pgMatches(spec *databasev1alpha1.PGUserAccessControl, observed *upcloud.ManagedDatabaseUserPGAccessControl) bool {
	if (spec == nil) != (observed == nil) {
		return false
	}
	if spec == nil {
		return true
	}
	if (spec.AllowReplication == nil) != (observed.AllowReplication == nil) {
		return false
	}
	if spec.AllowReplication != nil && *spec.AllowReplication != *observed.AllowReplication {
		return false
	}
	return true
}

func valkeyMatches(spec *databasev1alpha1.ValkeyUserAccessControl, observed *upcloud.ManagedDatabaseUserValkeyAccessControl) bool {
	if (spec == nil) != (observed == nil) {
		return false
	}
	if spec == nil {
		return true
	}
	return slicesEqual(spec.Categories, nilSlice(observed.Categories)) &&
		slicesEqual(spec.Channels, nilSlice(observed.Channels)) &&
		slicesEqual(spec.Commands, nilSlice(observed.Commands)) &&
		slicesEqual(spec.Keys, nilSlice(observed.Keys))
}

func openSearchMatches(spec *databasev1alpha1.OpenSearchUserAccessControl, observed *upcloud.ManagedDatabaseUserOpenSearchAccessControl) bool {
	if (spec == nil) != (observed == nil) {
		return false
	}
	if spec == nil {
		return true
	}
	want := make([]upcloud.ManagedDatabaseUserOpenSearchAccessControlRule, 0, len(spec.Rules))
	for _, r := range spec.Rules {
		want = append(want, upcloud.ManagedDatabaseUserOpenSearchAccessControlRule{
			Index:      r.Index,
			Permission: upcloud.ManagedDatabaseUserOpenSearchAccessControlRulePermission(r.Permission),
		})
	}
	var got []upcloud.ManagedDatabaseUserOpenSearchAccessControlRule
	if observed.Rules != nil {
		got = *observed.Rules
	}
	return reflect.DeepEqual(want, got)
}

func nilSlice(p *[]string) []string {
	if p == nil {
		return nil
	}
	return *p
}

func slicesEqual(a, b []string) bool { return reflect.DeepEqual(a, b) }

func pgRequest(spec *databasev1alpha1.PGUserAccessControl) *upcloud.ManagedDatabaseUserPGAccessControl {
	if spec == nil {
		return nil
	}
	return &upcloud.ManagedDatabaseUserPGAccessControl{AllowReplication: spec.AllowReplication}
}

func valkeyRequest(spec *databasev1alpha1.ValkeyUserAccessControl) *upcloud.ManagedDatabaseUserValkeyAccessControl {
	if spec == nil {
		return nil
	}
	out := &upcloud.ManagedDatabaseUserValkeyAccessControl{}
	if len(spec.Categories) > 0 {
		out.Categories = &spec.Categories
	}
	if len(spec.Channels) > 0 {
		out.Channels = &spec.Channels
	}
	if len(spec.Commands) > 0 {
		out.Commands = &spec.Commands
	}
	if len(spec.Keys) > 0 {
		out.Keys = &spec.Keys
	}
	return out
}

func openSearchRequest(spec *databasev1alpha1.OpenSearchUserAccessControl) *upcloud.ManagedDatabaseUserOpenSearchAccessControl {
	if spec == nil {
		return nil
	}
	rules := make([]upcloud.ManagedDatabaseUserOpenSearchAccessControlRule, 0, len(spec.Rules))
	for _, r := range spec.Rules {
		rules = append(rules, upcloud.ManagedDatabaseUserOpenSearchAccessControlRule{
			Index:      r.Index,
			Permission: upcloud.ManagedDatabaseUserOpenSearchAccessControlRulePermission(r.Permission),
		})
	}
	return &upcloud.ManagedDatabaseUserOpenSearchAccessControl{Rules: &rules}
}
