package loadbalancer

import (
	"context"
	"fmt"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// LoadBalancerCertificateBundleAdapter maps LoadBalancerCertificateBundle onto
// the UpCloud load balancer certificate bundle API.
//
// Bundles are account-level (not scoped to a load balancer). Manual and
// authority bundles read their certificate material from a kubernetes.io/tls
// Secret; dynamic bundles are provisioned by UpCloud and only take hostnames.
type LoadBalancerCertificateBundleAdapter struct {
	API    upcloudapi.LoadBalancerAPI
	Client client.Client
}

var _ reconciler.Adapter[*lb.LoadBalancerCertificateBundle] = (*LoadBalancerCertificateBundleAdapter)(nil)

// bundleSecretMaterial holds the certificate material read from a Secret.
type bundleSecretMaterial struct {
	Cert          string
	Key           string
	Intermediates string
}

// readBundleSecret reads the tls.crt / tls.key / ca.crt material from the
// referenced Secret (manual and authority bundles only).
func (a *LoadBalancerCertificateBundleAdapter) readBundleSecret(ctx context.Context, b *lb.LoadBalancerCertificateBundle) (*bundleSecretMaterial, error) {
	if b.Spec.CertificateSecretRef == nil {
		return nil, nil
	}
	secret := &corev1.Secret{}
	if err := a.Client.Get(ctx, types.NamespacedName{Namespace: b.Namespace, Name: b.Spec.CertificateSecretRef.Name}, secret); err != nil {
		return nil, fmt.Errorf("get certificate secret: %w", err)
	}
	return &bundleSecretMaterial{
		Cert:          string(secret.Data["tls.crt"]),
		Key:           string(secret.Data["tls.key"]),
		Intermediates: string(secret.Data["ca.crt"]),
	}, nil
}

// lookup finds the bundle by status UUID, or by name across the list (bundles
// carry no labels, so adoption is by Name).
func (a *LoadBalancerCertificateBundleAdapter) lookup(ctx context.Context, b *lb.LoadBalancerCertificateBundle) (*upcloud.LoadBalancerCertificateBundle, error) {
	if b.Status.UUID != "" {
		got, err := a.API.GetLoadBalancerCertificateBundle(ctx, &request.GetLoadBalancerCertificateBundleRequest{UUID: b.Status.UUID})
		if upcloudapi.IsNotFound(err) {
			return nil, nil
		}
		return got, err
	}
	list, err := a.API.GetLoadBalancerCertificateBundles(ctx, &request.GetLoadBalancerCertificateBundlesRequest{})
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].Name == b.ExternalName() {
			return &list[i], nil
		}
	}
	return nil, nil
}

// Observe implements reconciler.Adapter.
func (a *LoadBalancerCertificateBundleAdapter) Observe(ctx context.Context, b *lb.LoadBalancerCertificateBundle) (reconciler.Observation, error) {
	bundle, err := a.lookup(ctx, b)
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get certificate bundle: %w", err)
	}
	if bundle == nil {
		return reconciler.Observation{}, nil
	}
	b.Status.UUID = bundle.UUID
	a.syncStatus(b, bundle)

	var upToDate bool
	if b.Spec.Type == string(upcloud.LoadBalancerCertificateBundleTypeManual) || b.Spec.Type == string(upcloud.LoadBalancerCertificateBundleTypeAuthority) {
		material, err := a.readBundleSecret(ctx, b)
		if err != nil {
			return reconciler.Observation{}, err
		}
		// The API echoes the certificate (never the key), so up-to-date is
		// cert equality. Key rotation with an unchanged cert is undetectable.
		upToDate = material != nil && bundle.Certificate == material.Cert
	} else {
		// Dynamic bundles: the hostnames are fixed at create; existence means
		// up to date.
		upToDate = true
	}
	return reconciler.Observation{
		Exists:   true,
		UpToDate: upToDate,
		Ready:    bundle.OperationalState == upcloud.LoadBalancerCertificateBundleOperationalStateIdle,
		Message:  "state " + string(bundle.OperationalState),
	}, nil
}

// Create implements reconciler.Adapter. A Conflict (bundle already exists) is
// treated as adoption: the next Observe reconciles it.
func (a *LoadBalancerCertificateBundleAdapter) Create(ctx context.Context, b *lb.LoadBalancerCertificateBundle) error {
	req := &request.CreateLoadBalancerCertificateBundleRequest{
		Type: upcloud.LoadBalancerCertificateBundleType(b.Spec.Type),
		Name: b.ExternalName(),
	}
	switch b.Spec.Type {
	case string(upcloud.LoadBalancerCertificateBundleTypeDynamic):
		req.Hostnames = b.Spec.Hostnames
	default: // manual, authority
		material, err := a.readBundleSecret(ctx, b)
		if err != nil {
			return err
		}
		if material == nil {
			return fmt.Errorf("certificate bundle %q of type %s requires certificateSecretRef", b.Name, b.Spec.Type)
		}
		req.Certificate = material.Cert
		req.PrivateKey = material.Key
		req.Intermediates = material.Intermediates
	}
	created, err := a.API.CreateLoadBalancerCertificateBundle(ctx, req)
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles it
		}
		return fmt.Errorf("create certificate bundle: %w", err)
	}
	b.Status.UUID = created.UUID
	return nil
}

// Update implements reconciler.Adapter. Only manual and authority bundles are
// modifiable (cert / intermediates); dynamic bundles are not.
func (a *LoadBalancerCertificateBundleAdapter) Update(ctx context.Context, b *lb.LoadBalancerCertificateBundle) error {
	bundle, err := a.lookup(ctx, b)
	if err != nil {
		return fmt.Errorf("get certificate bundle: %w", err)
	}
	if bundle == nil {
		return fmt.Errorf("update certificate bundle: %w", reconciler.ErrPending)
	}
	if b.Spec.Type != string(upcloud.LoadBalancerCertificateBundleTypeManual) &&
		b.Spec.Type != string(upcloud.LoadBalancerCertificateBundleTypeAuthority) {
		return nil
	}
	material, err := a.readBundleSecret(ctx, b)
	if err != nil {
		return err
	}
	if material == nil {
		return nil
	}
	modify := &request.ModifyLoadBalancerCertificateBundleRequest{
		UUID:        bundle.UUID,
		Certificate: material.Cert,
		PrivateKey:  material.Key,
	}
	if material.Intermediates != "" {
		inter := material.Intermediates
		modify.Intermediates = &inter
	}
	if _, err := a.API.ModifyLoadBalancerCertificateBundle(ctx, modify); err != nil {
		return fmt.Errorf("modify certificate bundle: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A 404 counts as deleted.
func (a *LoadBalancerCertificateBundleAdapter) Delete(ctx context.Context, b *lb.LoadBalancerCertificateBundle) error {
	if b.Status.UUID == "" {
		return nil
	}
	err := a.API.DeleteLoadBalancerCertificateBundle(ctx, &request.DeleteLoadBalancerCertificateBundleRequest{UUID: b.Status.UUID})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	default:
		return fmt.Errorf("delete certificate bundle: %w", err)
	}
}

func (a *LoadBalancerCertificateBundleAdapter) syncStatus(b *lb.LoadBalancerCertificateBundle, bundle *upcloud.LoadBalancerCertificateBundle) {
	b.Status.OperationalState = string(bundle.OperationalState)
	b.Status.KeyType = bundle.KeyType
	if !bundle.NotAfter.IsZero() {
		b.Status.NotAfter = bundle.NotAfter.UTC().Format(time.RFC3339)
	}
	if !bundle.NotBefore.IsZero() {
		b.Status.NotBefore = bundle.NotBefore.UTC().Format(time.RFC3339)
	}
}
