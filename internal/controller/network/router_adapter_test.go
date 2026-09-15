package network

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newRouter() *networkv1alpha1.Router {
	return &networkv1alpha1.Router{
		ObjectMeta: metav1.ObjectMeta{Name: "r1", Namespace: "ns", UID: "router-uid"},
		Spec: networkv1alpha1.RouterSpec{
			StaticRoutes: []networkv1alpha1.StaticRoute{{Name: "to-office", Route: "10.9.0.0/16", Nexthop: "10.0.0.1"}},
			Labels:       map[string]string{"env": "test"},
		},
	}
}

func TestRouterObserveMissingThenCreate(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &RouterAdapter{API: api}
	r := newRouter()
	ctx := context.Background()

	obs, err := a.Observe(ctx, r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, r)).To(Succeed())
	g.Expect(r.Status.UUID).To(HavePrefix("rtr-"))
	created := api.Routers[r.Status.UUID]
	g.Expect(created.Name).To(Equal("r1"))
	g.Expect(created.StaticRoutes).To(Equal([]upcloud.StaticRoute{{Name: "to-office", Route: "10.9.0.0/16", Nexthop: "10.0.0.1", Type: upcloud.RouterStaticRouteTypeUser}}))
	g.Expect(upcloudapi.HasUID(created.Labels, "router-uid")).To(BeTrue())

	obs, err = a.Observe(ctx, r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs).To(Equal(reconciler.Observation{Exists: true, UpToDate: true, Ready: true}))
}

func TestRouterObserveAdoptsByLabelWhenStatusEmpty(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &RouterAdapter{API: api}
	r := newRouter()
	ctx := context.Background()
	g.Expect(a.Create(ctx, r)).To(Succeed())
	uuid := r.Status.UUID
	r.Status.UUID = ""

	obs, err := a.Observe(ctx, r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(r.Status.UUID).To(Equal(uuid))
}

func TestRouterObserveDetectsDriftAndUpdateFixesIt(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &RouterAdapter{API: api}
	r := newRouter()
	ctx := context.Background()
	g.Expect(a.Create(ctx, r)).To(Succeed())

	r.Spec.Name = "renamed"
	r.Spec.StaticRoutes = nil
	obs, err := a.Observe(ctx, r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, r)).To(Succeed())
	g.Expect(api.Routers[r.Status.UUID].Name).To(Equal("renamed"))
	g.Expect(api.Routers[r.Status.UUID].StaticRoutes).To(BeEmpty())
	obs, _ = a.Observe(ctx, r)
	g.Expect(obs.UpToDate).To(BeTrue())
}

func TestRouterObserveIgnoresServiceRoutes(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &RouterAdapter{API: api}
	r := newRouter()
	ctx := context.Background()
	g.Expect(a.Create(ctx, r)).To(Succeed())
	api.Routers[r.Status.UUID].StaticRoutes = append(api.Routers[r.Status.UUID].StaticRoutes,
		upcloud.StaticRoute{Route: "10.0.0.0/24", Nexthop: "10.0.0.254", Type: upcloud.RouterStaticRouteTypeService})
	obs, err := a.Observe(ctx, r)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())
}

func TestRouterDeleteIsIdempotent(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &RouterAdapter{API: api}
	r := newRouter()
	ctx := context.Background()
	g.Expect(a.Create(ctx, r)).To(Succeed())
	g.Expect(a.Delete(ctx, r)).To(Succeed())
	g.Expect(api.Routers).To(BeEmpty())
	g.Expect(a.Delete(ctx, r)).To(Succeed())

	g.Expect(a.Delete(ctx, &networkv1alpha1.Router{})).To(Succeed())
}

func TestRouterDeleteWithAttachedNetworkIsPending(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewNetworkAPI()
	a := &RouterAdapter{API: api}
	r := newRouter()
	ctx := context.Background()
	g.Expect(a.Create(ctx, r)).To(Succeed())
	api.Networks["net-x"] = &upcloud.Network{UUID: "net-x", Router: r.Status.UUID}
	err := a.Delete(ctx, r)
	g.Expect(err).To(MatchError(reconciler.ErrPending))
}
