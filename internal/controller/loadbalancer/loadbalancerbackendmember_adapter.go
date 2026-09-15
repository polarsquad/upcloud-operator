package loadbalancer

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// LoadBalancerBackendMemberAdapter maps LoadBalancerBackendMember onto the
// UpCloud load balancer backend member API. A member is addressed by
// (serviceUUID, backendName, name).
type LoadBalancerBackendMemberAdapter struct {
	API    upcloudapi.LoadBalancerAPI
	Client client.Client
}

var _ reconciler.Adapter[*lb.LoadBalancerBackendMember] = (*LoadBalancerBackendMemberAdapter)(nil)

// parent resolves the referenced LoadBalancerBackend (Ready) and stores its
// service UUID and backend name in status.
func (a *LoadBalancerBackendMemberAdapter) parent(ctx context.Context, m *lb.LoadBalancerBackendMember) error {
	if m.Status.ServiceUUID != "" && m.Status.BackendName != "" {
		return nil
	}
	be := &lb.LoadBalancerBackend{}
	if err := a.Client.Get(ctx, client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.BackendRef.Name}, be); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("backend %q not ready: %w", m.Spec.BackendRef.Name, reconciler.ErrDependencyNotReady)
		}
		return fmt.Errorf("get backend: %w", err)
	}
	if !reconciler.IsReady(be) || be.Status.ServiceUUID == "" || be.Status.Name == "" {
		return fmt.Errorf("backend %q not ready: %w", m.Spec.BackendRef.Name, reconciler.ErrDependencyNotReady)
	}
	m.Status.ServiceUUID = be.Status.ServiceUUID
	m.Status.BackendName = be.Status.Name
	return nil
}

// Observe implements reconciler.Adapter.
func (a *LoadBalancerBackendMemberAdapter) Observe(ctx context.Context, m *lb.LoadBalancerBackendMember) (reconciler.Observation, error) {
	if err := a.parent(ctx, m); err != nil {
		return reconciler.Observation{}, err
	}
	member, err := a.API.GetLoadBalancerBackendMember(ctx, &request.GetLoadBalancerBackendMemberRequest{
		ServiceUUID: m.Status.ServiceUUID, BackendName: m.Status.BackendName, Name: m.ExternalName(),
	})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get load balancer backend member: %w", err)
	}
	m.Status.Name = member.Name
	return reconciler.Observation{
		Exists:   true,
		UpToDate: memberMatches(*member, m),
		Ready:    true,
		Message:  "member " + member.Name,
	}, nil
}

// Create implements reconciler.Adapter. A Conflict is adoption.
func (a *LoadBalancerBackendMemberAdapter) Create(ctx context.Context, m *lb.LoadBalancerBackendMember) error {
	if err := a.parent(ctx, m); err != nil {
		return err
	}
	created, err := a.API.CreateLoadBalancerBackendMember(ctx, &request.CreateLoadBalancerBackendMemberRequest{
		ServiceUUID: m.Status.ServiceUUID,
		BackendName: m.Status.BackendName,
		Member:      desiredMember(m),
	})
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles it
		}
		return fmt.Errorf("create load balancer backend member: %w", err)
	}
	m.Status.Name = created.Name
	return nil
}

// Update implements reconciler.Adapter. Sends pointer fields for the changed
// values (an ip change is a modify, not a recreate).
func (a *LoadBalancerBackendMemberAdapter) Update(ctx context.Context, m *lb.LoadBalancerBackendMember) error {
	if err := a.parent(ctx, m); err != nil {
		return err
	}
	existing, err := a.API.GetLoadBalancerBackendMember(ctx, &request.GetLoadBalancerBackendMemberRequest{
		ServiceUUID: m.Status.ServiceUUID, BackendName: m.Status.BackendName, Name: m.ExternalName(),
	})
	if upcloudapi.IsNotFound(err) {
		return reconciler.ErrPending
	}
	if err != nil {
		return fmt.Errorf("get load balancer backend member: %w", err)
	}
	modify := request.ModifyLoadBalancerBackendMember{
		Type: upcloud.LoadBalancerBackendMemberType(m.Spec.Type),
		Port: m.Spec.Port,
	}
	if existing.IP != m.Spec.IP {
		ip := m.Spec.IP
		modify.IP = &ip
	}
	if existing.Weight != m.Spec.Weight {
		w := m.Spec.Weight
		modify.Weight = &w
	}
	if existing.MaxSessions != m.Spec.MaxSessions {
		ms := m.Spec.MaxSessions
		modify.MaxSessions = &ms
	}
	if existing.Enabled != m.Spec.Enabled {
		e := m.Spec.Enabled
		modify.Enabled = &e
	}
	if modify.Type == "" && modify.Port == 0 && modify.IP == nil && modify.Weight == nil &&
		modify.MaxSessions == nil && modify.Enabled == nil {
		return nil
	}
	if _, err := a.API.ModifyLoadBalancerBackendMember(ctx, &request.ModifyLoadBalancerBackendMemberRequest{
		ServiceUUID: m.Status.ServiceUUID, BackendName: m.Status.BackendName, Name: m.ExternalName(), Member: modify,
	}); err != nil {
		return fmt.Errorf("modify load balancer backend member: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A 404 counts as deleted.
func (a *LoadBalancerBackendMemberAdapter) Delete(ctx context.Context, m *lb.LoadBalancerBackendMember) error {
	if m.Status.ServiceUUID == "" || m.Status.BackendName == "" || m.Status.Name == "" {
		return nil
	}
	err := a.API.DeleteLoadBalancerBackendMember(ctx, &request.DeleteLoadBalancerBackendMemberRequest{
		ServiceUUID: m.Status.ServiceUUID, BackendName: m.Status.BackendName, Name: m.Status.Name,
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	default:
		return fmt.Errorf("delete load balancer backend member: %w", err)
	}
}

func desiredMember(m *lb.LoadBalancerBackendMember) request.LoadBalancerBackendMember {
	return request.LoadBalancerBackendMember{
		Name:        m.ExternalName(),
		Type:        upcloud.LoadBalancerBackendMemberType(m.Spec.Type),
		IP:          m.Spec.IP,
		Port:        m.Spec.Port,
		Weight:      m.Spec.Weight,
		MaxSessions: m.Spec.MaxSessions,
		Enabled:     m.Spec.Enabled,
	}
}

// memberMatches compares the member spec against the observed member.
func memberMatches(member upcloud.LoadBalancerBackendMember, m *lb.LoadBalancerBackendMember) bool {
	if member.Type != upcloud.LoadBalancerBackendMemberType(m.Spec.Type) ||
		member.IP != m.Spec.IP || member.Port != m.Spec.Port ||
		member.Weight != m.Spec.Weight || member.MaxSessions != m.Spec.MaxSessions ||
		member.Enabled != m.Spec.Enabled {
		return false
	}
	return true
}
