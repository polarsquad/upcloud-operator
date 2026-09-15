package loadbalancer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/polarsquad/upcloud-operator/api/common"
	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
)

// Object names are prefixed with "lb-" so this spec's objects do not collide
// with any other object in the shared envtest cluster.
const (
	envtestLB   = "lb-svc"
	envtestBE   = "lb-be"
	envtestMem  = "lb-member"
	envtestFE   = "lb-fe"
	envtestRule = "lb-rule"
	envtestBun  = "lb-bundle"
	envtestTls  = "lb-tls"
)

var _ = Describe("Load balancer group end to end against the fake API", func() {
	It("creates a service, backend, member, frontend, rule and bundle, then deletes them in reverse", func(ctx SpecContext) {
		// TLS secret for the manual certificate bundle.
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: envtestTls, Namespace: testNS},
			Type:       corev1.SecretTypeTLS,
			Data:       map[string][]byte{"tls.crt": []byte("CERT"), "tls.key": []byte("KEY")},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		svc := &lb.LoadBalancer{
			ObjectMeta: metav1.ObjectMeta{Name: envtestLB, Namespace: testNS},
			Spec: lb.LoadBalancerSpec{
				Zone: testZone,
				Plan: "0",
				Networks: []lb.LoadBalancerNetworkAttachment{
					{Name: "public", Type: "public"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, svc)).To(Succeed())

		// The service reaches Ready.
		Eventually(func(g Gomega) {
			var got lb.LoadBalancer
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(svc), &got)).To(Succeed())
			g.Expect(reconciler.IsReady(&got)).To(BeTrue())
			g.Expect(got.Status.UUID).NotTo(BeEmpty())
		}, "20s", "250ms").Should(Succeed())

		be := &lb.LoadBalancerBackend{
			ObjectMeta: metav1.ObjectMeta{Name: envtestBE, Namespace: testNS},
			Spec: lb.LoadBalancerBackendSpec{
				LoadBalancerRef: common.LocalObjectReference{Name: envtestLB},
				Name:            "api",
			},
		}
		Expect(k8sClient.Create(ctx, be)).To(Succeed())

		fe := &lb.LoadBalancerFrontend{
			ObjectMeta: metav1.ObjectMeta{Name: envtestFE, Namespace: testNS},
			Spec: lb.LoadBalancerFrontendSpec{
				LoadBalancerRef:   common.LocalObjectReference{Name: envtestLB},
				Name:              "web",
				Mode:              "http",
				Port:              80,
				DefaultBackendRef: common.LocalObjectReference{Name: envtestBE},
			},
		}
		Expect(k8sClient.Create(ctx, fe)).To(Succeed())

		// The backend and frontend reach Ready.
		Eventually(func(g Gomega) {
			var b lb.LoadBalancerBackend
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(be), &b)).To(Succeed())
			g.Expect(reconciler.IsReady(&b)).To(BeTrue())
			g.Expect(b.Status.Name).To(Equal("api"))

			var f lb.LoadBalancerFrontend
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(fe), &f)).To(Succeed())
			g.Expect(reconciler.IsReady(&f)).To(BeTrue())
		}, "20s", "250ms").Should(Succeed())

		// A backend member under the backend.
		member := &lb.LoadBalancerBackendMember{
			ObjectMeta: metav1.ObjectMeta{Name: envtestMem, Namespace: testNS},
			Spec: lb.LoadBalancerBackendMemberSpec{
				BackendRef: common.LocalObjectReference{Name: envtestBE},
				Name:       "node-1",
				Type:       "static",
				IP:         "10.0.0.5",
				Port:       8080,
				Weight:     1,
			},
		}
		Expect(k8sClient.Create(ctx, member)).To(Succeed())

		// A frontend rule with a src_ip matcher and a use_backend action.
		rule := &lb.LoadBalancerFrontendRule{
			ObjectMeta: metav1.ObjectMeta{Name: envtestRule, Namespace: testNS},
			Spec: lb.LoadBalancerFrontendRuleSpec{
				FrontendRef: common.LocalObjectReference{Name: envtestFE},
				Name:        "api",
				Priority:    10,
				Matchers: []lb.RuleMatcher{
					{Type: lb.MatcherTypeSrcIP, SrcIP: &lb.MatcherSrcIP{Value: "10.0.0.0/8"}},
				},
				Actions: []lb.RuleAction{
					{Type: lb.ActionTypeUseBackend, UseBackend: &lb.ActionUseBackend{Backend: "api"}},
				},
			},
		}
		Expect(k8sClient.Create(ctx, rule)).To(Succeed())

		// A manual certificate bundle backed by the TLS secret.
		bundle := &lb.LoadBalancerCertificateBundle{
			ObjectMeta: metav1.ObjectMeta{Name: envtestBun, Namespace: testNS},
			Spec: lb.LoadBalancerCertificateBundleSpec{
				Name:                 "api-cert",
				Type:                 "manual",
				CertificateSecretRef: &common.LocalObjectReference{Name: envtestTls},
			},
		}
		Expect(k8sClient.Create(ctx, bundle)).To(Succeed())

		// The member, rule and bundle all reach Ready.
		Eventually(func(g Gomega) {
			var m lb.LoadBalancerBackendMember
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(member), &m)).To(Succeed())
			g.Expect(reconciler.IsReady(&m)).To(BeTrue())

			var r lb.LoadBalancerFrontendRule
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(rule), &r)).To(Succeed())
			g.Expect(reconciler.IsReady(&r)).To(BeTrue())

			var b lb.LoadBalancerCertificateBundle
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bundle), &b)).To(Succeed())
			g.Expect(reconciler.IsReady(&b)).To(BeTrue())
			g.Expect(b.Status.UUID).NotTo(BeEmpty())
		}, "20s", "250ms").Should(Succeed())

		// Delete in reverse: rule, member, bundle, frontend, backend, service.
		Expect(k8sClient.Delete(ctx, rule)).To(Succeed())
		Expect(k8sClient.Delete(ctx, member)).To(Succeed())
		Expect(k8sClient.Delete(ctx, bundle)).To(Succeed())
		Expect(k8sClient.Delete(ctx, fe)).To(Succeed())
		Expect(k8sClient.Delete(ctx, be)).To(Succeed())
		Expect(k8sClient.Delete(ctx, svc)).To(Succeed())

		// Deleting the whole chain empties the fake.
		Eventually(func() int {
			total := len(fakeAPI.LoadBalancers)
			for _, lbw := range fakeAPI.LoadBalancers {
				total += len(lbw.Frontends)
				total += len(lbw.Backends)
				total += len(lbw.Resolvers)
				for _, f := range lbw.Frontends {
					total += len(f.Rules)
					total += len(f.TLSConfigs)
				}
				for _, b := range lbw.Backends {
					total += len(b.Members)
					total += len(b.TLSConfigs)
				}
			}
			total += len(fakeAPI.CertificateBundles)
			return total
		}, "20s", "250ms").Should(BeZero())
	})
})
