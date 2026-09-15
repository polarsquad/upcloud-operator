// Package resolve turns CR references into UpCloud identifiers.
package resolve

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
)

// Ready loads the referenced object into obj and returns its external ID.
// It fails with ErrDependencyNotReady unless the object exists, has an
// external ID and its Ready condition is True.
func Ready(ctx context.Context, c client.Client, namespace, name string, obj reconciler.Object) (string, error) {
	id, err := ExternalID(ctx, c, namespace, name, obj)
	if err != nil {
		return "", err
	}
	if !reconciler.IsReady(obj) {
		return "", fmt.Errorf("%w: %T %q is not Ready", reconciler.ErrDependencyNotReady, obj, name)
	}
	return id, nil
}

// ExternalID loads the referenced object into obj and returns its external
// ID without requiring Ready. Use it in Observe for children whose parent may
// be mid-update.
func ExternalID(ctx context.Context, c client.Client, namespace, name string, obj reconciler.Object) (string, error) {
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("%w: %T %q not found", reconciler.ErrDependencyNotReady, obj, name)
		}
		return "", err
	}
	if obj.GetExternalID() == "" {
		return "", fmt.Errorf("%w: %T %q has no external id yet", reconciler.ErrDependencyNotReady, obj, name)
	}
	return obj.GetExternalID(), nil
}

// NetworkUUID resolves a NetworkAttachment to an UpCloud network UUID.
// Public and utility attachments resolve to "".
func NetworkUUID(ctx context.Context, c client.Client, namespace string, att common.NetworkAttachment) (string, error) {
	switch {
	case att.UUID != "":
		return att.UUID, nil
	case att.NetworkRef != nil:
		return Ready(ctx, c, namespace, att.NetworkRef.Name, &networkv1alpha1.Network{})
	case att.Type == "private":
		return "", fmt.Errorf("network attachment %q: type private requires uuid or networkRef", att.Name)
	default:
		return "", nil
	}
}

// SecretKey reads one key from a Secret. Missing Secret or key is a dependency error.
func SecretKey(ctx context.Context, c client.Client, namespace string, sel common.SecretKeySelector) (string, error) {
	var s corev1.Secret
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: sel.Name}, &s); err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("%w: Secret %q not found", reconciler.ErrDependencyNotReady, sel.Name)
		}
		return "", err
	}
	v, ok := s.Data[sel.Key]
	if !ok {
		return "", fmt.Errorf("%w: Secret %q has no key %q", reconciler.ErrDependencyNotReady, sel.Name, sel.Key)
	}
	return string(v), nil
}
