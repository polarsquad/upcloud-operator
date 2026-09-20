package objectstorage

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/k8s"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// ObjectStorageAccessKeyAdapter maps ObjectStorageAccessKey onto the UpCloud
// object storage access key API. The S3 secret is only returned once, at
// create time, and is written to an owned Secret in the same call.
type ObjectStorageAccessKeyAdapter struct {
	API    upcloudapi.ObjectStorageAPI
	Client client.Client
}

var _ reconciler.Adapter[*objectstoragev1alpha1.ObjectStorageAccessKey] = (*ObjectStorageAccessKeyAdapter)(nil)

// parentUsername resolves the referenced ObjectStorageUser (which must be
// Ready) and stores the service UUID and username in Status.
func (a *ObjectStorageAccessKeyAdapter) parent(ctx context.Context, k *objectstoragev1alpha1.ObjectStorageAccessKey) (string, string, error) {
	if k.Status.ServiceUUID != "" && k.Status.Username != "" {
		return k.Status.ServiceUUID, k.Status.Username, nil
	}
	user := &objectstoragev1alpha1.ObjectStorageUser{}
	if err := a.Client.Get(ctx, types.NamespacedName{Namespace: k.Namespace, Name: k.Spec.UserRef.Name}, user); err != nil {
		return "", "", fmt.Errorf("%w: ObjectStorageUser %q not found", reconciler.ErrDependencyNotReady, k.Spec.UserRef.Name)
	}
	if user.GetExternalID() == "" || !reconciler.IsReady(user) {
		return "", "", fmt.Errorf("%w: ObjectStorageUser %q not ready", reconciler.ErrDependencyNotReady, k.Spec.UserRef.Name)
	}
	k.Status.ServiceUUID = user.Status.ServiceUUID
	k.Status.Username = user.Status.Username
	return k.Status.ServiceUUID, k.Status.Username, nil
}

// Observe implements reconciler.Adapter. A missing access key (or an empty
// AccessKeyID, which cannot be adopted) reports Exists: false.
func (a *ObjectStorageAccessKeyAdapter) Observe(ctx context.Context, k *objectstoragev1alpha1.ObjectStorageAccessKey) (reconciler.Observation, error) {
	if k.Status.AccessKeyID == "" {
		return reconciler.Observation{}, nil
	}
	svcUUID, username, err := a.parent(ctx, k)
	if err != nil {
		return reconciler.Observation{}, err
	}
	key, err := a.API.GetManagedObjectStorageUserAccessKey(ctx, &request.GetManagedObjectStorageUserAccessKeyRequest{
		ServiceUUID: svcUUID,
		Username:    username,
		AccessKeyID: k.Status.AccessKeyID,
	})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get object storage access key: %w", err)
	}
	if !key.CreatedAt.IsZero() {
		k.Status.CreatedAt = key.CreatedAt.Format("2006-01-02T15:04:05Z")
	}
	desired := upcloud.ManagedObjectStorageUserAccessKeyStatus("Active")
	if k.Spec.Status != "" {
		desired = upcloud.ManagedObjectStorageUserAccessKeyStatus(k.Spec.Status)
	}
	upToDate := key.Status == desired
	return reconciler.Observation{
		Exists:   true,
		UpToDate: upToDate,
		Ready:    true,
		Message:  "access key " + key.AccessKeyID,
	}, nil
}

// Create implements reconciler.Adapter. It creates the key, stores the
// AccessKeyID, and writes the S3 Secret in the same call. If the Secret write
// fails, the key is deleted so the next pass recreates it cleanly (the secret
// is only returned once and would otherwise be lost).
func (a *ObjectStorageAccessKeyAdapter) Create(ctx context.Context, k *objectstoragev1alpha1.ObjectStorageAccessKey) error {
	svcUUID, username, err := a.parent(ctx, k)
	if err != nil {
		return err
	}
	created, err := a.API.CreateManagedObjectStorageUserAccessKey(ctx, &request.CreateManagedObjectStorageUserAccessKeyRequest{
		ServiceUUID: svcUUID,
		Username:    username,
	})
	if err != nil {
		return fmt.Errorf("create object storage access key: %w", err)
	}
	k.Status.AccessKeyID = created.AccessKeyID
	if !created.CreatedAt.IsZero() {
		k.Status.CreatedAt = created.CreatedAt.Format("2006-01-02T15:04:05Z")
	}

	secret, err := a.secretData(ctx, svcUUID, created.AccessKeyID, created.SecretAccessKey)
	if err != nil {
		_ = a.deleteKey(ctx, svcUUID, username, created.AccessKeyID)
		return err
	}
	if err := k8s.WriteOwnedSecret(ctx, a.Client, k, k.SecretKeyName(), secret); err != nil {
		_ = a.deleteKey(ctx, svcUUID, username, created.AccessKeyID)
		k.Status.AccessKeyID = ""
		return fmt.Errorf("write S3 secret: %w", err)
	}
	return nil
}

// secretData builds the S3 credentials Secret data. endpoint is the public
// endpoint domain of the parent service; region is the parent region.
func (a *ObjectStorageAccessKeyAdapter) secretData(ctx context.Context, svcUUID, accessKeyID string, secret *string) (map[string][]byte, error) {
	svc, err := a.API.GetManagedObjectStorage(ctx, &request.GetManagedObjectStorageRequest{UUID: svcUUID})
	if err != nil {
		return nil, fmt.Errorf("get object storage service: %w", err)
	}
	endpoint := ""
	for _, e := range svc.Endpoints {
		if e.Type == EndpointTypePublic {
			endpoint = "https://" + e.DomainName
			break
		}
	}
	secretVal := ""
	if secret != nil {
		secretVal = *secret
	}
	return map[string][]byte{
		SecretKeyAccessKeyID:     []byte(accessKeyID),
		SecretKeySecretAccessKey: []byte(secretVal),
		SecretKeyEndpointURL:     []byte(endpoint),
		SecretKeyRegion:          []byte(svc.Region),
	}, nil
}

// Update implements reconciler.Adapter. Status drift is pushed with
// ModifyManagedObjectStorageUserAccessKey.
func (a *ObjectStorageAccessKeyAdapter) Update(ctx context.Context, k *objectstoragev1alpha1.ObjectStorageAccessKey) error {
	if k.Status.AccessKeyID == "" {
		return reconciler.ErrPending
	}
	svcUUID, username, err := a.parent(ctx, k)
	if err != nil {
		return err
	}
	desired := upcloud.ManagedObjectStorageUserAccessKeyStatus("Active")
	if k.Spec.Status != "" {
		desired = upcloud.ManagedObjectStorageUserAccessKeyStatus(k.Spec.Status)
	}
	if _, err := a.API.ModifyManagedObjectStorageUserAccessKey(ctx, &request.ModifyManagedObjectStorageUserAccessKeyRequest{
		ServiceUUID: svcUUID,
		Username:    username,
		AccessKeyID: k.Status.AccessKeyID,
		Status:      desired,
	}); err != nil {
		return fmt.Errorf("modify object storage access key: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. Deletes the key; a 404 counts as
// deleted and a conflict is pending.
func (a *ObjectStorageAccessKeyAdapter) Delete(ctx context.Context, k *objectstoragev1alpha1.ObjectStorageAccessKey) error {
	if k.Status.AccessKeyID == "" || k.Status.ServiceUUID == "" || k.Status.Username == "" {
		return nil
	}
	return a.deleteKey(ctx, k.Status.ServiceUUID, k.Status.Username, k.Status.AccessKeyID)
}

func (a *ObjectStorageAccessKeyAdapter) deleteKey(ctx context.Context, svcUUID, username, accessKeyID string) error {
	err := a.API.DeleteManagedObjectStorageUserAccessKey(ctx, &request.DeleteManagedObjectStorageUserAccessKeyRequest{
		ServiceUUID: svcUUID,
		Username:    username,
		AccessKeyID: accessKeyID,
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		return fmt.Errorf("%w: %s", reconciler.ErrPending, upcloudapi.TitleOf(err))
	default:
		return fmt.Errorf("delete object storage access key: %w", err)
	}
}
