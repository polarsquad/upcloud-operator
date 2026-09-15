package loadbalancer

import (
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/polarsquad/upcloud-operator/api/common"
	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

// tlsSecret creates a kubernetes.io/tls Secret holding the given cert/key.
func tlsSecret(name, cert, key string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
		Type:       corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte(cert),
			"tls.key": []byte(key),
		},
	}
}

func TestBundleCreateManualDriftDelete(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerCertificateBundleAdapter{API: api, Client: c}
	ctx := gctx()

	sec := tlsSecret(tlsSecretName, certPEM, "KEY")
	g.Expect(c.Create(ctx, sec)).To(Succeed())

	b := &lb.LoadBalancerCertificateBundle{
		ObjectMeta: metav1.ObjectMeta{Name: "b1", Namespace: testNS},
		Spec: lb.LoadBalancerCertificateBundleSpec{
			Name:                 "b1",
			Type:                 "manual",
			CertificateSecretRef: &common.LocalObjectReference{Name: tlsSecretName},
		},
	}
	g.Expect(a.Create(ctx, b)).To(Succeed())
	g.Expect(b.Status.UUID).NotTo(BeEmpty())

	obs, err := a.Observe(ctx, b)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(obs.Ready).To(BeTrue())    // manual bundle is idle
	g.Expect(obs.UpToDate).To(BeTrue()) // cert matches the secret

	// Drift: change the cert in the secret.
	sec.Data["tls.crt"] = []byte("CHANGED")
	g.Expect(c.Update(ctx, sec)).To(Succeed())
	obs, err = a.Observe(ctx, b)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	g.Expect(a.Update(ctx, b)).To(Succeed())
	obs, err = a.Observe(ctx, b)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())

	g.Expect(a.Delete(ctx, b)).To(Succeed())
	g.Expect(a.Delete(ctx, b)).To(Succeed())
}

// TestBundleAdopt guards name-based adoption (no UUID in status).
func TestBundleAdopt(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerCertificateBundleAdapter{API: api, Client: c}
	ctx := gctx()

	// A bundle already exists in UpCloud with this name.
	api.CertificateBundles["cb-existing"] = &upcloud.LoadBalancerCertificateBundle{
		UUID: "cb-existing", Name: "b2", Type: upcloud.LoadBalancerCertificateBundleTypeManual,
		OperationalState: upcloud.LoadBalancerCertificateBundleOperationalStateIdle,
	}

	b := &lb.LoadBalancerCertificateBundle{
		ObjectMeta: metav1.ObjectMeta{Name: "b2", Namespace: testNS},
		Spec:       lb.LoadBalancerCertificateBundleSpec{Name: "b2", Type: bundleType, CertificateSecretRef: &common.LocalObjectReference{Name: tlsSecretName}},
	}
	sec := tlsSecret(tlsSecretName, "CERT", "KEY")
	g.Expect(c.Create(ctx, sec)).To(Succeed())

	obs, err := a.Observe(ctx, b)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.Exists).To(BeTrue())
	g.Expect(b.Status.UUID).To(Equal("cb-existing"))
}

// TestBundleRotationTokenForcesKeyReapply covers the case the API cannot
// detect: a key-only rotation (unchanged cert) plus a token bump.
func TestBundleRotationTokenForcesKeyReapply(t *testing.T) {
	g := NewWithT(t)
	api := fake.NewLoadBalancerAPI()
	c := newLBClient(t)
	a := &LoadBalancerCertificateBundleAdapter{API: api, Client: c}
	ctx := gctx()

	sec := tlsSecret(tlsSecretName, certPEM, "KEY")
	g.Expect(c.Create(ctx, sec)).To(Succeed())

	b := &lb.LoadBalancerCertificateBundle{
		ObjectMeta: metav1.ObjectMeta{Name: "b1", Namespace: testNS},
		Spec: lb.LoadBalancerCertificateBundleSpec{
			Name:                 "b1",
			Type:                 bundleType,
			CertificateSecretRef: &common.LocalObjectReference{Name: tlsSecretName},
			RotationToken:        "v1",
		},
	}
	g.Expect(a.Create(ctx, b)).To(Succeed())
	g.Expect(b.Status.RotationToken).To(Equal("v1"))

	// Key-only rotation: the cert is unchanged, so the API-level check
	// (cert equality) still says up to date.
	sec.Data["tls.key"] = []byte("NEWKEY")
	g.Expect(c.Update(ctx, sec)).To(Succeed())
	obs, err := a.Observe(ctx, b)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())

	// Bumping the token is the operator-side signal for the key change.
	b.Spec.RotationToken = "v2"
	obs, err = a.Observe(ctx, b)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeFalse())

	// Update re-applies the material and records the new token.
	g.Expect(a.Update(ctx, b)).To(Succeed())
	g.Expect(b.Status.RotationToken).To(Equal("v2"))
	g.Expect(api.Calls).To(ContainElement("ModifyLoadBalancerCertificateBundle"))

	// Steady state again: token recorded, cert matches.
	obs, err = a.Observe(ctx, b)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(obs.UpToDate).To(BeTrue())
}
