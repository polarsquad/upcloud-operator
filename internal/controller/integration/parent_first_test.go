// Package integration_test exercises the real generic reconciler and adapters
// together. The Kubernetes fake implements status subresources, deletion
// timestamps and finalizer removal; the UpCloud fakes implement the cloud API.
// Reconcile calls are scheduled deterministically rather than by a manager:
// every pending pass must explicitly request a retry before we deliver it.
// No test deletes cloud state or strips a finalizer to make progress.
package integration_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8sfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	databasev1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	networkv1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	objectstoragev1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
)

const retryDelay = 11 * time.Millisecond

func pairClient(t *testing.T) client.Client {
	t.Helper()
	g := NewWithT(t)
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())
	g.Expect(networkv1.AddToScheme(s)).To(Succeed())
	g.Expect(objectstoragev1.AddToScheme(s)).To(Succeed())
	g.Expect(databasev1.AddToScheme(s)).To(Succeed())
	return k8sfake.NewClientBuilder().WithScheme(s).WithStatusSubresource(
		&networkv1.Router{}, &networkv1.Network{},
		&objectstoragev1.ManagedObjectStorage{}, &objectstoragev1.ObjectStorageBucket{},
		&databasev1.ManagedDatabase{}, &databasev1.ManagedDatabaseUser{},
	).Build()
}

func pairMeta(name string) metav1.ObjectMeta {
	// The fake client does not allocate UIDs. Distinct UIDs exercise cloud
	// ownership labels and owned Secrets just as API-server-assigned UIDs do.
	return metav1.ObjectMeta{Name: name, Namespace: "default", UID: types.UID(name), Generation: 1}
}

func reconcileOnce[T reconciler.Object](t *testing.T, r *reconciler.Reconciler[T], obj T) ctrl.Result {
	t.Helper()
	result, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(obj)})
	NewWithT(t).Expect(err).NotTo(HaveOccurred())
	return result
}

func createReady[T reconciler.Object](t *testing.T, r *reconciler.Reconciler[T], obj T) {
	t.Helper()
	g := NewWithT(t)
	g.Expect(r.Client.Create(context.Background(), obj)).To(Succeed())
	for range 10 {
		reconcileOnce(t, r, obj)
		g.Expect(r.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)).To(Succeed())
		if reconciler.IsReady(obj) {
			break
		}
	}
	g.Expect(reconciler.IsReady(obj)).To(BeTrue(), "real adapter must create and observe the resource as Ready")
	g.Expect(obj.GetExternalID()).NotTo(BeEmpty())
	g.Expect(obj.GetFinalizers()).To(ContainElement(r.Finalizer))
}

func requestDeletion(t *testing.T, c client.Client, obj client.Object) {
	t.Helper()
	g := NewWithT(t)
	g.Expect(c.Delete(context.Background(), obj)).To(Succeed())
	g.Expect(c.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)).To(Succeed())
	g.Expect(obj.GetDeletionTimestamp().IsZero()).To(BeFalse())
}

func requirePending[T reconciler.Object](t *testing.T, r *reconciler.Reconciler[T], obj T) {
	t.Helper()
	g := NewWithT(t)
	id := obj.GetExternalID()
	result := reconcileOnce(t, r, obj)
	g.Expect(result.RequeueAfter).To(Equal(retryDelay), "pending deletion must schedule another reconcile")
	g.Expect(r.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)).To(Succeed(), "pending CR must remain")
	g.Expect(obj.GetDeletionTimestamp().IsZero()).To(BeFalse())
	g.Expect(obj.GetFinalizers()).To(ContainElement(r.Finalizer), "pending deletion must retain the real finalizer")
	g.Expect(obj.GetExternalID()).To(Equal(id), "retry must retain the external identity")
	condition := meta.FindStatusCondition(obj.GetConditions(), reconciler.ConditionReady)
	g.Expect(condition).NotTo(BeNil())
	g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
	g.Expect(condition.Reason).To(Equal(reconciler.ReasonDeleting))
}

func requireGone[T reconciler.Object](t *testing.T, r *reconciler.Reconciler[T], obj T) {
	t.Helper()
	g := NewWithT(t)
	g.Expect(reconcileOnce(t, r, obj)).To(Equal(ctrl.Result{}))
	g.Expect(apierrors.IsNotFound(r.Get(context.Background(), client.ObjectKeyFromObject(obj), r.New()))).To(BeTrue(),
		"CR must disappear through its real finalizer, not through test cleanup")
}

func requireLive(t *testing.T, c client.Client, obj client.Object) {
	t.Helper()
	g := NewWithT(t)
	g.Expect(c.Get(context.Background(), client.ObjectKeyFromObject(obj), obj)).To(Succeed())
	g.Expect(obj.GetDeletionTimestamp().IsZero()).To(BeTrue(), "child must still be live when parent deletion starts")
}

func deleteCalls(calls []string) []string {
	// These tests are single-threaded: no manager accesses the fakes in the
	// background. Cloud observations use locking API methods, never maps.
	var deletes []string
	for _, call := range calls {
		if len(call) >= len("Delete") && call[:len("Delete")] == "Delete" {
			deletes = append(deletes, call)
		}
	}
	return deletes
}
