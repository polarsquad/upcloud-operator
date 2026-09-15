package objectstorage

import (
	"context"
	"fmt"
	"slices"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"sigs.k8s.io/controller-runtime/pkg/client"

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/resolve"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// ObjectStorageUserAdapter maps ObjectStorageUser onto the UpCloud object
// storage user API. The adapter also reconciles the user's attached policies.
type ObjectStorageUserAdapter struct {
	API    upcloudapi.ObjectStorageAPI
	Client client.Client
}

var _ reconciler.Adapter[*objectstoragev1alpha1.ObjectStorageUser] = (*ObjectStorageUserAdapter)(nil)

// serviceUUID resolves the referenced ManagedObjectStorage (which must be
// Ready) on first use and stores the result in Status.
func (a *ObjectStorageUserAdapter) serviceUUID(ctx context.Context, u *objectstoragev1alpha1.ObjectStorageUser) (string, error) {
	if u.Status.ServiceUUID != "" {
		return u.Status.ServiceUUID, nil
	}
	uuid, err := resolve.Ready(ctx, a.Client, u.Namespace, u.Spec.ServiceRef.Name, &objectstoragev1alpha1.ManagedObjectStorage{})
	if err != nil {
		return "", err
	}
	u.Status.ServiceUUID = uuid
	return uuid, nil
}

// desiredPolicies resolves the spec policy names. Plain entries are used as
// is; policyRefs are resolved to the referenced ObjectStoragePolicy's
// status.name (requiring it to be Ready). The result is a sorted, de-duped
// list for stable comparison.
func (a *ObjectStorageUserAdapter) desiredPolicies(ctx context.Context, u *objectstoragev1alpha1.ObjectStorageUser) ([]string, error) {
	set := make(map[string]bool)
	for _, name := range u.Spec.Policies {
		set[name] = true
	}
	for _, ref := range u.Spec.PolicyRefs {
		name, err := a.policyRefName(ctx, u, ref.Name)
		if err != nil {
			return nil, err
		}
		set[name] = true
	}
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	slices.Sort(out)
	return out, nil
}

// policyRefName resolves a policyRef to its policy name. The referenced
// ObjectStoragePolicy must be Ready and its name is its external id.
func (a *ObjectStorageUserAdapter) policyRefName(ctx context.Context, u *objectstoragev1alpha1.ObjectStorageUser, refName string) (string, error) {
	name, err := resolve.Ready(ctx, a.Client, u.Namespace, refName, &objectstoragev1alpha1.ObjectStoragePolicy{})
	if err != nil {
		return "", err
	}
	return name, nil
}

// currentPolicies returns the policy names currently attached in UpCloud,
// sorted for stable comparison.
func (a *ObjectStorageUserAdapter) currentPolicies(ctx context.Context, svcUUID, username string) ([]string, error) {
	list, err := a.API.GetManagedObjectStorageUserPolicies(ctx, &request.GetManagedObjectStorageUserPoliciesRequest{ServiceUUID: svcUUID, Username: username})
	if err != nil {
		return nil, fmt.Errorf("get object storage user policies: %w", err)
	}
	out := make([]string, 0, len(list))
	for _, p := range list {
		out = append(out, p.Name)
	}
	slices.Sort(out)
	return out, nil
}

// Observe implements reconciler.Adapter. A missing user reports Exists: false.
func (a *ObjectStorageUserAdapter) Observe(ctx context.Context, u *objectstoragev1alpha1.ObjectStorageUser) (reconciler.Observation, error) {
	svcUUID, err := a.serviceUUID(ctx, u)
	if err != nil {
		return reconciler.Observation{}, err
	}
	user, err := a.API.GetManagedObjectStorageUser(ctx, &request.GetManagedObjectStorageUserRequest{ServiceUUID: svcUUID, Username: u.ExternalUsername()})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get object storage user: %w", err)
	}
	u.Status.Username = user.Username
	u.Status.ARN = user.ARN

	attached, err := a.currentPolicies(ctx, svcUUID, user.Username)
	if err != nil {
		return reconciler.Observation{}, err
	}
	u.Status.AttachedPolicies = attached

	desired, err := a.desiredPolicies(ctx, u)
	if err != nil {
		return reconciler.Observation{}, err
	}
	upToDate := equalStringSlices(attached, desired)
	return reconciler.Observation{
		Exists:   true,
		UpToDate: upToDate,
		Ready:    true,
		Message:  "user " + user.Username,
	}, nil
}

// Create implements reconciler.Adapter. A Conflict (user already exists in
// UpCloud) is treated as adoption: the next Observe reconciles it, including
// the policies.
func (a *ObjectStorageUserAdapter) Create(ctx context.Context, u *objectstoragev1alpha1.ObjectStorageUser) error {
	svcUUID, err := a.serviceUUID(ctx, u)
	if err != nil {
		return err
	}
	created, err := a.API.CreateManagedObjectStorageUser(ctx, &request.CreateManagedObjectStorageUserRequest{
		ServiceUUID: svcUUID,
		Username:    u.ExternalUsername(),
	})
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles policies
		}
		return fmt.Errorf("create object storage user: %w", err)
	}
	u.Status.Username = created.Username
	u.Status.ARN = created.ARN
	return a.attachDesired(ctx, u, svcUUID)
}

// attachDesired attaches every desired policy that is not yet attached.
func (a *ObjectStorageUserAdapter) attachDesired(ctx context.Context, u *objectstoragev1alpha1.ObjectStorageUser, svcUUID string) error {
	desired, err := a.desiredPolicies(ctx, u)
	if err != nil {
		return err
	}
	attached, err := a.currentPolicies(ctx, svcUUID, u.ExternalUsername())
	if err != nil {
		return err
	}
	for _, name := range desired {
		if containsString(attached, name) {
			continue
		}
		if err := a.API.AttachManagedObjectStorageUserPolicy(ctx, &request.AttachManagedObjectStorageUserPolicyRequest{
			ServiceUUID: svcUUID,
			Username:    u.ExternalUsername(),
			Name:        name,
		}); err != nil {
			return fmt.Errorf("attach policy %q: %w", name, err)
		}
	}
	return nil
}

// Update implements reconciler.Adapter. Attaches missing policies and detaches
// extras, one API call each.
func (a *ObjectStorageUserAdapter) Update(ctx context.Context, u *objectstoragev1alpha1.ObjectStorageUser) error {
	svcUUID, err := a.serviceUUID(ctx, u)
	if err != nil {
		return err
	}
	if _, err := a.API.GetManagedObjectStorageUser(ctx, &request.GetManagedObjectStorageUserRequest{ServiceUUID: svcUUID, Username: u.ExternalUsername()}); upcloudapi.IsNotFound(err) {
		return reconciler.ErrPending
	} else if err != nil {
		return fmt.Errorf("get object storage user: %w", err)
	}

	desired, err := a.desiredPolicies(ctx, u)
	if err != nil {
		return err
	}
	attached, err := a.currentPolicies(ctx, svcUUID, u.ExternalUsername())
	if err != nil {
		return err
	}
	for _, name := range desired {
		if containsString(attached, name) {
			continue
		}
		if err := a.API.AttachManagedObjectStorageUserPolicy(ctx, &request.AttachManagedObjectStorageUserPolicyRequest{
			ServiceUUID: svcUUID,
			Username:    u.ExternalUsername(),
			Name:        name,
		}); err != nil {
			return fmt.Errorf("attach policy %q: %w", name, err)
		}
	}
	for _, name := range attached {
		if containsString(desired, name) {
			continue
		}
		if err := a.API.DetachManagedObjectStorageUserPolicy(ctx, &request.DetachManagedObjectStorageUserPolicyRequest{
			ServiceUUID: svcUUID,
			Username:    u.ExternalUsername(),
			Name:        name,
		}); err != nil {
			return fmt.Errorf("detach policy %q: %w", name, err)
		}
	}
	return nil
}

// Delete implements reconciler.Adapter. Detaches every attached policy,
// deletes the user's access keys, then deletes the user; a 404 counts as
// deleted.
func (a *ObjectStorageUserAdapter) Delete(ctx context.Context, u *objectstoragev1alpha1.ObjectStorageUser) error {
	if u.Status.ServiceUUID == "" || u.Status.Username == "" {
		return nil
	}
	user, err := a.API.GetManagedObjectStorageUser(ctx, &request.GetManagedObjectStorageUserRequest{
		ServiceUUID: u.Status.ServiceUUID,
		Username:    u.Status.Username,
	})
	if upcloudapi.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get object storage user: %w", err)
	}
	// Detach every attached policy.
	for _, p := range user.Policies {
		if err := a.API.DetachManagedObjectStorageUserPolicy(ctx, &request.DetachManagedObjectStorageUserPolicyRequest{
			ServiceUUID: u.Status.ServiceUUID,
			Username:    u.Status.Username,
			Name:        p.Name,
		}); err != nil && !upcloudapi.IsNotFound(err) {
			return fmt.Errorf("detach policy %q: %w", p.Name, err)
		}
	}
	// Delete every access key.
	for _, k := range user.AccessKeys {
		if err := a.API.DeleteManagedObjectStorageUserAccessKey(ctx, &request.DeleteManagedObjectStorageUserAccessKeyRequest{
			ServiceUUID: u.Status.ServiceUUID,
			Username:    u.Status.Username,
			AccessKeyID: k.AccessKeyID,
		}); err != nil && !upcloudapi.IsNotFound(err) {
			return fmt.Errorf("delete access key %q: %w", k.AccessKeyID, err)
		}
	}
	if err := a.API.DeleteManagedObjectStorageUser(ctx, &request.DeleteManagedObjectStorageUserRequest{
		ServiceUUID: u.Status.ServiceUUID,
		Username:    u.Status.Username,
	}); err != nil && !upcloudapi.IsNotFound(err) {
		return fmt.Errorf("delete object storage user: %w", err)
	}
	return nil
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsString(s []string, v string) bool {
	return slices.Contains(s, v)
}
