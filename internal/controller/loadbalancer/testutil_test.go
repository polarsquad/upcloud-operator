package loadbalancer

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"

	"github.com/polarsquad/upcloud-operator/api/common"
	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

// Shared test constants. Kept in one place so the goconst linter (which
// counts occurrences per package) does not flag repeated literals.
const (
	testNS          = "default"
	parentName      = "svc"
	parentUUID      = "lb-0001"
	testPlan        = "0-4"
	testZone        = "fi-hel1"
	readyReason     = "Available"
	testReadyType   = "Ready"
	publicType      = "public"
	bundleType      = "manual"
	testMethodExact = "exact"
	testAPIPath     = "/api"
)

// newLBClient returns a fake client with the corev1, network and loadbalancer
// schemes, for adapter unit tests.
func newLBClient(t *testing.T) client.Client {
	g := NewWithT(t)
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())
	g.Expect(lb.AddToScheme(s)).To(Succeed())
	g.Expect(networkv1alpha1.AddToScheme(s)).To(Succeed())
	return fakeclient.NewClientBuilder().WithScheme(s).Build()
}

// gctx returns a background context for test helpers.
func gctx() context.Context { return context.Background() }

// commonLocalRef returns a common.LocalObjectReference for a name, for test
// fixtures.
func commonLocalRef(name string) common.LocalObjectReference {
	return common.LocalObjectReference{Name: name}
}

const (
	backendName = "be"
	feName      = "fe"
	ruleName    = "rule"
	bundleName  = "bundle"
	bundleUUID  = "cb-0001"
	certPEM     = "CERT"
)

// readyFrontend creates a Ready LoadBalancerFrontend CR named feName and the
// matching fake frontend (under the ready parent), for rule / frontend TLS
// config tests.
func readyFrontend(g *GomegaWithT, c client.Client, api *fake.LoadBalancerAPI) {
	readyBackend(g, c, api)
	fe := &lb.LoadBalancerFrontend{
		ObjectMeta: metav1.ObjectMeta{Name: feName, Namespace: testNS, UID: types.UID(feName), Generation: 1},
		Spec: lb.LoadBalancerFrontendSpec{
			LoadBalancerRef:   commonLocalRef(parentName),
			Name:              feName,
			Mode:              "http",
			Port:              80,
			DefaultBackendRef: commonLocalRef(backendName),
			Networks:          []lb.FrontendNetwork{{Name: publicType}},
		},
		Status: lb.LoadBalancerFrontendStatus{
			ServiceUUID: parentUUID,
			Name:        feName,
			Conditions:  []metav1.Condition{{Type: testReadyType, Status: metav1.ConditionTrue, Reason: readyReason, Message: "ok", ObservedGeneration: 1}},
		},
	}
	g.Expect(c.Create(gctx(), fe)).To(Succeed())
	api.LoadBalancers[parentUUID].Frontends = append(api.LoadBalancers[parentUUID].Frontends,
		upcloud.LoadBalancerFrontend{Name: feName, Mode: upcloud.LoadBalancerModeHTTP, Port: 80, DefaultBackend: backendName})
}

// readyBackend creates a Ready LoadBalancerBackend CR named backendName and the
// matching fake backend (under the ready parent), for member / TLS config
// tests.
func readyBackend(g *GomegaWithT, c client.Client, api *fake.LoadBalancerAPI) {
	readyService(g, c, api)
	be := &lb.LoadBalancerBackend{
		ObjectMeta: metav1.ObjectMeta{Name: backendName, Namespace: testNS, UID: types.UID(backendName), Generation: 1},
		Spec:       lb.LoadBalancerBackendSpec{LoadBalancerRef: commonLocalRef(parentName), Name: backendName},
		Status: lb.LoadBalancerBackendStatus{
			ServiceUUID: parentUUID,
			Name:        backendName,
			Conditions:  []metav1.Condition{{Type: testReadyType, Status: metav1.ConditionTrue, Reason: readyReason, Message: "ok", ObservedGeneration: 1}},
		},
	}
	g.Expect(c.Create(gctx(), be)).To(Succeed())
	api.LoadBalancers[parentUUID].Backends = append(api.LoadBalancers[parentUUID].Backends,
		upcloud.LoadBalancerBackend{Name: backendName})
}

// readyBundle creates a Ready LoadBalancerCertificateBundle CR named
// bundleName and the matching fake bundle, for TLS config tests.
func readyBundle(g *GomegaWithT, c client.Client, api *fake.LoadBalancerAPI) {
	b := &lb.LoadBalancerCertificateBundle{
		ObjectMeta: metav1.ObjectMeta{Name: bundleName, Namespace: testNS, UID: types.UID(bundleName), Generation: 1},
		Spec:       lb.LoadBalancerCertificateBundleSpec{Name: bundleName, Type: bundleType},
		Status: lb.LoadBalancerCertificateBundleStatus{
			UUID:             bundleUUID,
			OperationalState: "idle",
			Conditions:       []metav1.Condition{{Type: testReadyType, Status: metav1.ConditionTrue, Reason: readyReason, Message: "ok", ObservedGeneration: 1}},
		},
	}
	g.Expect(c.Create(gctx(), b)).To(Succeed())
	api.CertificateBundles[bundleUUID] = &upcloud.LoadBalancerCertificateBundle{
		UUID:             bundleUUID,
		Name:             bundleName,
		Type:             upcloud.LoadBalancerCertificateBundleTypeManual,
		OperationalState: upcloud.LoadBalancerCertificateBundleOperationalStateIdle,
		Certificate:      certPEM,
	}
}

// readyService creates a Ready LoadBalancer CR named parentName and the
// matching fake service, for child adapter tests.
func readyService(g *GomegaWithT, c client.Client, api *fake.LoadBalancerAPI) {
	svc := &lb.LoadBalancer{
		ObjectMeta: metav1.ObjectMeta{Name: parentName, Namespace: testNS, UID: types.UID(parentName), Generation: 1},
		Spec: lb.LoadBalancerSpec{
			Plan: testPlan,
			Zone: testZone,
			Networks: []lb.LoadBalancerNetworkAttachment{
				{Name: publicType, Type: publicType},
			},
		},
		Status: lb.LoadBalancerStatus{
			UUID:             parentUUID,
			OperationalState: "running",
			Conditions:       []metav1.Condition{{Type: testReadyType, Status: metav1.ConditionTrue, Reason: readyReason, Message: "ok", ObservedGeneration: 1}},
		},
	}
	g.Expect(c.Create(gctx(), svc)).To(Succeed())
	api.LoadBalancers[parentUUID] = &upcloud.LoadBalancer{
		UUID:             parentUUID,
		Name:             parentName,
		Zone:             testZone,
		Plan:             testPlan,
		OperationalState: upcloud.LoadBalancerOperationalStateRunning,
		ConfiguredStatus: upcloud.LoadBalancerConfiguredStatusStarted,
		Networks: []upcloud.LoadBalancerNetwork{
			{Name: publicType, Type: upcloud.LoadBalancerNetworkTypePublic, Family: upcloud.LoadBalancerAddressFamilyIPv4, DNSName: parentUUID + ".lb.upcloud.com"},
		},
	}
}
