package reconciler

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
)

func TestDependentsOfEnqueuesOnlyMatchingChildren(t *testing.T) {
	g := NewWithT(t)
	s := runtime.NewScheme()
	g.Expect(networkv1alpha1.AddToScheme(s)).To(Succeed())

	n1 := &networkv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "n1", Namespace: "ns"},
		Spec:       networkv1alpha1.NetworkSpec{Zone: "fi-hel1", RouterRef: &common.LocalObjectReference{Name: "r1"}},
	}
	n2 := &networkv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "n2", Namespace: "ns"},
		Spec:       networkv1alpha1.NetworkSpec{Zone: "fi-hel1", RouterRef: &common.LocalObjectReference{Name: "other"}},
	}
	n3 := &networkv1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "n3", Namespace: "ns"},
		Spec:       networkv1alpha1.NetworkSpec{Zone: "fi-hel1"},
	}
	c := fakeclient.NewClientBuilder().WithScheme(s).WithObjects(n1, n2, n3).Build()

	router := &networkv1alpha1.Router{ObjectMeta: metav1.ObjectMeta{Name: "r1", Namespace: "ns"}}
	mf := DependentsOf(c,
		func() *networkv1alpha1.NetworkList { return &networkv1alpha1.NetworkList{} },
		func(l *networkv1alpha1.NetworkList) []*networkv1alpha1.Network {
			out := make([]*networkv1alpha1.Network, 0, len(l.Items))
			for i := range l.Items {
				out = append(out, &l.Items[i])
			}
			return out
		},
		func(n *networkv1alpha1.Network) []string {
			if n.Spec.RouterRef == nil {
				return nil
			}
			return []string{n.Spec.RouterRef.Name}
		},
	)

	reqs := mf(context.Background(), router)
	g.Expect(reqs).To(HaveLen(1))
	g.Expect(reqs[0].Name).To(Equal("n1"))
	g.Expect(reqs[0].Namespace).To(Equal("ns"))
}
