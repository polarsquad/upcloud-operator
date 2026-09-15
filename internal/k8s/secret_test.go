package k8s_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/k8s"
)

func TestWriteOwnedSecretCreatesThenUpdates(t *testing.T) {
	g := NewWithT(t)
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())
	g.Expect(networkv1alpha1.AddToScheme(s)).To(Succeed())
	owner := &networkv1alpha1.Network{ObjectMeta: metav1.ObjectMeta{Name: "o", Namespace: "ns", UID: "u1"}}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(owner).Build()
	ctx := context.Background()

	g.Expect(k8s.WriteOwnedSecret(ctx, c, owner, "o-conn", map[string][]byte{"a": []byte("1")})).To(Succeed())
	var sec corev1.Secret
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "ns", Name: "o-conn"}, &sec)).To(Succeed())
	g.Expect(sec.Data["a"]).To(Equal([]byte("1")))
	g.Expect(sec.OwnerReferences).To(HaveLen(1))
	g.Expect(sec.OwnerReferences[0].UID).To(Equal(types.UID("u1")))
	g.Expect(*sec.OwnerReferences[0].Controller).To(BeTrue())

	g.Expect(k8s.WriteOwnedSecret(ctx, c, owner, "o-conn", map[string][]byte{"a": []byte("2")})).To(Succeed())
	g.Expect(c.Get(ctx, types.NamespacedName{Namespace: "ns", Name: "o-conn"}, &sec)).To(Succeed())
	g.Expect(sec.Data["a"]).To(Equal([]byte("2")))
}
