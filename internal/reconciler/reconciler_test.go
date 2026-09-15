package reconciler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
)

const finalizer = "test.upcloud.polarsquad.com/finalizer"

var key = types.NamespacedName{Namespace: "default", Name: "n1"}

type fakeAdapter struct {
	obs                       reconciler.Observation
	obsErr, updateErr, delErr error
	creates, updates, deletes int
}

func (f *fakeAdapter) Observe(context.Context, *networkv1alpha1.Network) (reconciler.Observation, error) {
	return f.obs, f.obsErr
}
func (f *fakeAdapter) Create(_ context.Context, n *networkv1alpha1.Network) error {
	f.creates++
	n.Status.UUID = "created-uuid"
	return nil
}
func (f *fakeAdapter) Update(context.Context, *networkv1alpha1.Network) error {
	f.updates++
	return f.updateErr
}
func (f *fakeAdapter) Delete(context.Context, *networkv1alpha1.Network) error {
	f.deletes++
	return f.delErr
}

func network(policy common.DeletionPolicy, finalizers ...string) *networkv1alpha1.Network {
	return &networkv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace, Finalizers: finalizers, Generation: 3},
		Spec:       networkv1alpha1.NetworkSpec{Zone: "fi-hel1", DeletionPolicy: policy},
	}
}

func harness(t *testing.T, obj *networkv1alpha1.Network, fa *fakeAdapter) (client.Client, *reconciler.Reconciler[*networkv1alpha1.Network]) {
	t.Helper()
	s := runtime.NewScheme()
	NewWithT(t).Expect(networkv1alpha1.AddToScheme(s)).To(Succeed())
	c := fake.NewClientBuilder().WithScheme(s).
		WithStatusSubresource(&networkv1alpha1.Network{}).WithObjects(obj).Build()
	r := &reconciler.Reconciler[*networkv1alpha1.Network]{
		Client: c, Adapter: fa, Finalizer: finalizer,
		New: func() *networkv1alpha1.Network { return &networkv1alpha1.Network{} },
	}
	return c, r
}

func reconcile(r *reconciler.Reconciler[*networkv1alpha1.Network]) (ctrl.Result, error) {
	return r.Reconcile(context.Background(), ctrl.Request{NamespacedName: key})
}

func get(t *testing.T, c client.Client) *networkv1alpha1.Network {
	t.Helper()
	var n networkv1alpha1.Network
	NewWithT(t).Expect(c.Get(context.Background(), key, &n)).To(Succeed())
	return &n
}

func ready(t *testing.T, c client.Client) *metav1.Condition {
	t.Helper()
	cond := meta.FindStatusCondition(get(t, c).Status.Conditions, reconciler.ConditionReady)
	NewWithT(t).Expect(cond).NotTo(BeNil())
	return cond
}

func TestFirstPassOnlyAddsFinalizer(t *testing.T) {
	g := NewWithT(t)
	fa := &fakeAdapter{}
	c, r := harness(t, network(common.DeletionPolicyDelete), fa)
	_, err := reconcile(r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(get(t, c).Finalizers).To(ContainElement(finalizer))
	g.Expect(fa.creates).To(BeZero())
}

func TestMissingResourceIsCreated(t *testing.T) {
	g := NewWithT(t)
	fa := &fakeAdapter{obs: reconciler.Observation{Exists: false}}
	c, r := harness(t, network(common.DeletionPolicyDelete, finalizer), fa)
	res, err := reconcile(r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fa.creates).To(Equal(1))
	g.Expect(res.RequeueAfter).To(Equal(15 * time.Second))
	g.Expect(get(t, c).Status.UUID).To(Equal("created-uuid"))
	cond := ready(t, c)
	g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
	g.Expect(cond.Reason).To(Equal(reconciler.ReasonCreating))
	g.Expect(cond.ObservedGeneration).To(Equal(int64(3)))
}

func TestDriftTriggersUpdate(t *testing.T) {
	g := NewWithT(t)
	fa := &fakeAdapter{obs: reconciler.Observation{Exists: true, UpToDate: false, Ready: true}}
	c, r := harness(t, network(common.DeletionPolicyDelete, finalizer), fa)
	res, err := reconcile(r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fa.updates).To(Equal(1))
	g.Expect(res.RequeueAfter).To(Equal(15 * time.Second))
	g.Expect(ready(t, c).Reason).To(Equal(reconciler.ReasonUpdating))
}

func TestUpdatePendingIsNotAnError(t *testing.T) {
	g := NewWithT(t)
	fa := &fakeAdapter{obs: reconciler.Observation{Exists: true}, updateErr: reconciler.ErrPending}
	c, r := harness(t, network(common.DeletionPolicyDelete, finalizer), fa)
	res, err := reconcile(r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res.RequeueAfter).To(Equal(15 * time.Second))
	g.Expect(ready(t, c).Reason).To(Equal(reconciler.ReasonPending))
}

func TestInSyncButNotReadyWaits(t *testing.T) {
	g := NewWithT(t)
	fa := &fakeAdapter{obs: reconciler.Observation{Exists: true, UpToDate: true, Ready: false, Message: "state rebuilding"}}
	c, r := harness(t, network(common.DeletionPolicyDelete, finalizer), fa)
	res, err := reconcile(r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fa.updates).To(BeZero())
	g.Expect(res.RequeueAfter).To(Equal(15 * time.Second))
	cond := ready(t, c)
	g.Expect(cond.Reason).To(Equal(reconciler.ReasonPending))
	g.Expect(cond.Message).To(Equal("state rebuilding"))
}

func TestReadyAndInSyncIsAvailable(t *testing.T) {
	g := NewWithT(t)
	fa := &fakeAdapter{obs: reconciler.Observation{Exists: true, UpToDate: true, Ready: true}}
	c, r := harness(t, network(common.DeletionPolicyDelete, finalizer), fa)
	res, err := reconcile(r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res.RequeueAfter).To(Equal(5 * time.Minute))
	cond := ready(t, c)
	g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
	g.Expect(cond.Reason).To(Equal(reconciler.ReasonAvailable))
}

func TestDependencyNotReadyRequeuesQuietly(t *testing.T) {
	g := NewWithT(t)
	fa := &fakeAdapter{obsErr: errors.Join(reconciler.ErrDependencyNotReady, errors.New(`Router "r1" not found`))}
	c, r := harness(t, network(common.DeletionPolicyDelete, finalizer), fa)
	res, err := reconcile(r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res.RequeueAfter).To(Equal(15 * time.Second))
	cond := ready(t, c)
	g.Expect(cond.Reason).To(Equal(reconciler.ReasonWaitingForDependency))
	g.Expect(cond.Message).To(ContainSubstring("r1"))
}

func TestObserveErrorIsReturnedAndRecorded(t *testing.T) {
	g := NewWithT(t)
	fa := &fakeAdapter{obsErr: errors.New("boom")}
	c, r := harness(t, network(common.DeletionPolicyDelete, finalizer), fa)
	_, err := reconcile(r)
	g.Expect(err).To(MatchError(ContainSubstring("boom")))
	g.Expect(ready(t, c).Reason).To(Equal(reconciler.ReasonObserveFailed))
}

func TestDeleteKeepsFinalizerWhilePending(t *testing.T) {
	g := NewWithT(t)
	fa := &fakeAdapter{delErr: reconciler.ErrPending}
	c, r := harness(t, network(common.DeletionPolicyDelete, finalizer), fa)
	g.Expect(c.Delete(context.Background(), get(t, c))).To(Succeed())
	res, err := reconcile(r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fa.deletes).To(Equal(1))
	g.Expect(res.RequeueAfter).To(Equal(15 * time.Second))
	g.Expect(get(t, c).Finalizers).To(ContainElement(finalizer))
	g.Expect(ready(t, c).Reason).To(Equal(reconciler.ReasonDeleting))

	fa.delErr = nil
	_, err = reconcile(r)
	g.Expect(err).NotTo(HaveOccurred())
	var n networkv1alpha1.Network
	g.Expect(apierrors.IsNotFound(c.Get(context.Background(), key, &n))).To(BeTrue())
}

func TestOrphanPolicySkipsExternalDelete(t *testing.T) {
	g := NewWithT(t)
	fa := &fakeAdapter{}
	c, r := harness(t, network(common.DeletionPolicyOrphan, finalizer), fa)
	g.Expect(c.Delete(context.Background(), get(t, c))).To(Succeed())
	_, err := reconcile(r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fa.deletes).To(BeZero())
	var n networkv1alpha1.Network
	g.Expect(apierrors.IsNotFound(c.Get(context.Background(), key, &n))).To(BeTrue())
}

func TestDeleteFailureIsRecorded(t *testing.T) {
	g := NewWithT(t)
	fa := &fakeAdapter{delErr: errors.New("termination protection enabled")}
	c, r := harness(t, network(common.DeletionPolicyDelete, finalizer), fa)
	g.Expect(c.Delete(context.Background(), get(t, c))).To(Succeed())
	_, err := reconcile(r)
	g.Expect(err).To(HaveOccurred())
	g.Expect(ready(t, c).Reason).To(Equal(reconciler.ReasonDeleteFailed))
	g.Expect(get(t, c).Finalizers).To(ContainElement(finalizer))
}

func TestGoneObjectIsNoop(t *testing.T) {
	g := NewWithT(t)
	fa := &fakeAdapter{}
	_, r := harness(t, network(common.DeletionPolicyDelete, finalizer), fa)
	r.Client = fake.NewClientBuilder().WithScheme(r.Client.Scheme()).Build()
	res, err := reconcile(r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).To(Equal(ctrl.Result{}))
}
