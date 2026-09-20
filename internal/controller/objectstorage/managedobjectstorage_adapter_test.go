package objectstorage

import (
	"context"
	"errors"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func newService(name string) *objectstoragev1alpha1.ManagedObjectStorage {
	return &objectstoragev1alpha1.ManagedObjectStorage{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS, UID: "svc-uid"},
		Spec:       objectstoragev1alpha1.ManagedObjectStorageSpec{Region: regionFinland},
	}
}

func TestServiceCreateDefaults(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	a := &ManagedObjectStorageAdapter{API: api, Client: c}
	s := newService("bucket-svc")
	ctx := context.Background()

	obs, err := a.Observe(ctx, s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeFalse())

	g.Expect(a.Create(ctx, s)).To(Succeed())
	g.Expect(s.Status.UUID).NotTo(BeEmpty())

	// The fake assigns a public endpoint from the UUID.
	svc := api.Services[s.Status.UUID]
	g.Expect(svc).NotTo(BeNil())
	g.Expect(svc.Region).To(Equal(regionFinland))
	g.Expect(svc.OperationalState).To(Equal(upcloud.ManagedObjectStorageOperationalStateRunning))
	g.Expect(svc.Endpoints).To(HaveLen(1))
	g.Expect(svc.Endpoints[0].Type).To(Equal(EndpointTypePublic))
	g.Expect(svc.Endpoints[0].DomainName).To(HaveSuffix(".upcloudobjects.com"))
	g.Expect(svc.Name).To(Equal("bucket-svc"))

	// Owner labels are present.
	g.Expect(svc.Labels).To(ContainElement(upcloud.Label{Key: upcloudapi.LabelManagedBy, Value: upcloudapi.ManagedByValue}))
	// Observe now finds it, is ready and up to date.
	obs, err = a.Observe(ctx, s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())
	g.Expect(obs.UpToDate).To(BeTrue())
	g.Expect(s.Status.OperationalState).To(Equal("running"))
	g.Expect(s.Status.Endpoints).To(HaveLen(1))
}

func TestServiceNetworkRefResolution(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	a := &ManagedObjectStorageAdapter{API: api, Client: c}
	s := newService("net-svc")
	ctx := context.Background()

	// Create first so the service exists in the fake.
	g.Expect(a.Create(ctx, s)).To(Succeed())

	// Now drift the spec to a private network whose NetworkRef points at a
	// missing Network CR; that is a dependency error.
	s.Spec.Networks = []objectstoragev1alpha1.MOSNetworkAttachment{
		{Name: "priv", Type: "private", NetworkRef: &common.LocalObjectReference{Name: "nonexistent"}},
	}
	_, err := a.Observe(ctx, s)
	g.Expect(err).To(MatchError(ContainSubstring("Network")))
	g.Expect(errors.Is(err, reconciler.ErrDependencyNotReady)).To(BeTrue())
}

func TestServiceStatusFlipStopped(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	a := &ManagedObjectStorageAdapter{API: api, Client: c}
	s := newService("stop-svc")
	s.Spec.ConfiguredStatus = "stopped"
	ctx := context.Background()

	g.Expect(a.Create(ctx, s)).To(Succeed())
	svc := api.Services[s.Status.UUID]
	g.Expect(svc.ConfiguredStatus).To(Equal(upcloud.ManagedObjectStorageConfiguredStatusStopped))
	g.Expect(svc.OperationalState).To(Equal(upcloud.ManagedObjectStorageOperationalStateStopped))

	// A stopped service is Ready because the spec asked for stopped.
	obs, err := a.Observe(ctx, s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Ready).To(BeTrue())
}

func TestServiceDriftOnLabelsTriggersUpdate(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	a := &ManagedObjectStorageAdapter{API: api, Client: c}
	s := newService("label-svc")
	ctx := context.Background()

	g.Expect(a.Create(ctx, s)).To(Succeed())

	// Drift: the spec gains a user label the service does not have.
	s.Spec.Labels = common.UpCloudLabels{"team": "platform"}
	obs, err := a.Observe(ctx, s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, s)).To(Succeed())
	got := api.Services[s.Status.UUID].Labels
	g.Expect(got).To(ContainElement(upcloud.Label{Key: "team", Value: "platform"}))

	obs, err = a.Observe(ctx, s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())
}

func TestServiceDeletePendingUntilGone(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	a := &ManagedObjectStorageAdapter{API: api, Client: c}
	s := newService("del-svc")
	ctx := context.Background()

	g.Expect(a.Create(ctx, s)).To(Succeed())
	uuid := s.Status.UUID

	// First delete request succeeds and is pending until the service is gone.
	g.Expect(a.Delete(ctx, s)).To(MatchError(reconciler.ErrPending))
	// The fake removes the service immediately, so a second pass sees 404.
	g.Expect(a.Delete(ctx, s)).To(Succeed())
	g.Expect(api.Services[uuid]).To(BeNil())
}

func TestServiceAdoptByUID(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewObjectStorageAPI()
	c := newMOSClient(t)
	a := &ManagedObjectStorageAdapter{API: api, Client: c}
	s := newService("adopt-svc")
	ctx := context.Background()

	// A service already exists in UpCloud owned by this CR (by UID label) but
	// the CR has no status UUID yet. Create must not be called; Observe adopts.
	adopted := &upcloud.ManagedObjectStorage{
		UUID:             "moss-adopt",
		Name:             "adopt-svc",
		Region:           regionFinland,
		OperationalState: upcloud.ManagedObjectStorageOperationalStateRunning,
		ConfiguredStatus: upcloud.ManagedObjectStorageConfiguredStatusStarted,
		Labels:           []upcloud.Label{{Key: upcloudapi.LabelUID, Value: "svc-uid"}},
		Endpoints:        []upcloud.ManagedObjectStorageEndpoint{{DomainName: "moss-adopt.upcloudobjects.com", Type: EndpointTypePublic}},
	}
	api.Services[adopted.UUID] = adopted

	obs, err := a.Observe(ctx, s)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(s.Status.UUID).To(Equal("moss-adopt"))
}
