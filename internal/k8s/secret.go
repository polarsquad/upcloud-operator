// Package k8s holds small Kubernetes-side helpers used by adapters.
package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// WriteOwnedSecret creates or updates an Opaque Secret in the owner's
// namespace with a controller owner reference, so it is garbage collected
// with the CR.
func WriteOwnedSecret(ctx context.Context, c client.Client, owner client.Object, name string, data map[string][]byte) error {
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: owner.GetNamespace()}}
	_, err := controllerutil.CreateOrUpdate(ctx, c, sec, func() error {
		sec.Type = corev1.SecretTypeOpaque
		sec.Data = data
		return controllerutil.SetControllerReference(owner, sec, c.Scheme())
	})
	return err
}
