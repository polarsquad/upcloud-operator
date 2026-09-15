package resolve_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/resolve"
)

func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	s := runtime.NewScheme()
	NewWithT(t).Expect(networkv1alpha1.AddToScheme(s)).To(Succeed())
	NewWithT(t).Expect(corev1.AddToScheme(s)).To(Succeed())
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
}

func readyNetwork(uuid string) *networkv1alpha1.Network {
	n := &networkv1alpha1.Network{ObjectMeta: metav1.ObjectMeta{Name: "net", Namespace: "ns"}}
	n.Status.UUID = uuid
	if uuid != "" {
		reconciler.SetReady(n, metav1.ConditionTrue, reconciler.ReasonAvailable, "")
	}
	return n
}

func TestReadyReturnsExternalID(t *testing.T) {
	g := NewWithT(t)
	c := newClient(t, readyNetwork("uuid-1"))
	id, err := resolve.Ready(context.Background(), c, "ns", "net", &networkv1alpha1.Network{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(id).To(Equal("uuid-1"))
}

func TestReadyMissingObjectIsDependencyError(t *testing.T) {
	g := NewWithT(t)
	_, err := resolve.Ready(context.Background(), newClient(t), "ns", "net", &networkv1alpha1.Network{})
	g.Expect(err).To(MatchError(reconciler.ErrDependencyNotReady))
	g.Expect(err.Error()).To(ContainSubstring(`"net"`))
}

func TestReadyNotReadyObjectIsDependencyError(t *testing.T) {
	g := NewWithT(t)
	c := newClient(t, readyNetwork(""))
	_, err := resolve.Ready(context.Background(), c, "ns", "net", &networkv1alpha1.Network{})
	g.Expect(err).To(MatchError(reconciler.ErrDependencyNotReady))
}

func TestExternalIDDoesNotRequireReady(t *testing.T) {
	g := NewWithT(t)
	n := readyNetwork("uuid-2")
	n.Status.Conditions = nil
	id, err := resolve.ExternalID(context.Background(), newClient(t, n), "ns", "net", &networkv1alpha1.Network{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(id).To(Equal("uuid-2"))
}

func TestNetworkUUIDVariants(t *testing.T) {
	g := NewWithT(t)
	c := newClient(t, readyNetwork("uuid-3"))
	ctx := context.Background()

	id, err := resolve.NetworkUUID(ctx, c, "ns", common.NetworkAttachment{Type: "private", UUID: "literal"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(id).To(Equal("literal"))

	id, err = resolve.NetworkUUID(ctx, c, "ns", common.NetworkAttachment{Type: "private", NetworkRef: &common.LocalObjectReference{Name: "net"}})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(id).To(Equal("uuid-3"))

	id, err = resolve.NetworkUUID(ctx, c, "ns", common.NetworkAttachment{Type: "public"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(id).To(BeEmpty())

	_, err = resolve.NetworkUUID(ctx, c, "ns", common.NetworkAttachment{Name: "p", Type: "private"})
	g.Expect(err).To(MatchError(ContainSubstring("requires uuid or networkRef")))
	g.Expect(err).NotTo(MatchError(reconciler.ErrDependencyNotReady))
}

func TestSecretKey(t *testing.T) {
	g := NewWithT(t)
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "s", Namespace: "ns"}, Data: map[string][]byte{"password": []byte("pw")}}
	c := newClient(t, sec)
	v, err := resolve.SecretKey(context.Background(), c, "ns", common.SecretKeySelector{Name: "s", Key: "password"})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(v).To(Equal("pw"))

	_, err = resolve.SecretKey(context.Background(), c, "ns", common.SecretKeySelector{Name: "s", Key: "missing"})
	g.Expect(err).To(MatchError(reconciler.ErrDependencyNotReady))
	_, err = resolve.SecretKey(context.Background(), c, "ns", common.SecretKeySelector{Name: "nope", Key: "k"})
	g.Expect(err).To(MatchError(reconciler.ErrDependencyNotReady))
}
