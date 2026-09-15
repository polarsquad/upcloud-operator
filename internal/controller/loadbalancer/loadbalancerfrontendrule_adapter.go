package loadbalancer

import (
	"context"
	"fmt"
	"reflect"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	lb "github.com/polarsquad/upcloud-operator/api/loadbalancer/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// LoadBalancerFrontendRuleAdapter maps LoadBalancerFrontendRule onto the
// UpCloud load balancer frontend rule API. A rule is addressed by
// (serviceUUID, frontendName, name) and has no modify endpoint: updates use a
// full replace.
type LoadBalancerFrontendRuleAdapter struct {
	API    upcloudapi.LoadBalancerAPI
	Client client.Client
}

var _ reconciler.Adapter[*lb.LoadBalancerFrontendRule] = (*LoadBalancerFrontendRuleAdapter)(nil)

// parent resolves the referenced LoadBalancerFrontend (Ready) and stores its
// service UUID and frontend name in status.
func (a *LoadBalancerFrontendRuleAdapter) parent(ctx context.Context, r *lb.LoadBalancerFrontendRule) error {
	if r.Status.ServiceUUID != "" && r.Status.FrontendName != "" {
		return nil
	}
	fe := &lb.LoadBalancerFrontend{}
	if err := a.Client.Get(ctx, client.ObjectKey{Namespace: r.Namespace, Name: r.Spec.FrontendRef.Name}, fe); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("frontend %q not ready: %w", r.Spec.FrontendRef.Name, reconciler.ErrDependencyNotReady)
		}
		return fmt.Errorf("get frontend: %w", err)
	}
	if !reconciler.IsReady(fe) || fe.Status.ServiceUUID == "" || fe.Status.Name == "" {
		return fmt.Errorf("frontend %q not ready: %w", r.Spec.FrontendRef.Name, reconciler.ErrDependencyNotReady)
	}
	r.Status.ServiceUUID = fe.Status.ServiceUUID
	r.Status.FrontendName = fe.Status.Name
	return nil
}

// Observe implements reconciler.Adapter.
func (a *LoadBalancerFrontendRuleAdapter) Observe(ctx context.Context, r *lb.LoadBalancerFrontendRule) (reconciler.Observation, error) {
	if err := a.parent(ctx, r); err != nil {
		return reconciler.Observation{}, err
	}
	obs, err := a.API.GetLoadBalancerFrontendRule(ctx, &request.GetLoadBalancerFrontendRuleRequest{
		ServiceUUID: r.Status.ServiceUUID, FrontendName: r.Status.FrontendName, Name: r.ExternalName(),
	})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get load balancer frontend rule: %w", err)
	}
	r.Status.Name = obs.Name
	return reconciler.Observation{
		Exists:   true,
		UpToDate: ruleUpToDate(obs, toRule(r)),
		Ready:    true,
		Message:  "frontend rule " + obs.Name,
	}, nil
}

// Create implements reconciler.Adapter. A Conflict is adoption.
func (a *LoadBalancerFrontendRuleAdapter) Create(ctx context.Context, r *lb.LoadBalancerFrontendRule) error {
	if err := a.parent(ctx, r); err != nil {
		return err
	}
	created, err := a.API.CreateLoadBalancerFrontendRule(ctx, &request.CreateLoadBalancerFrontendRuleRequest{
		ServiceUUID:  r.Status.ServiceUUID,
		FrontendName: r.Status.FrontendName,
		Rule:         toRule(r),
	})
	if err != nil {
		if upcloudapi.IsConflict(err) {
			return nil // adopted; the next Observe reconciles it
		}
		return fmt.Errorf("create load balancer frontend rule: %w", err)
	}
	r.Status.Name = created.Name
	return nil
}

// Update implements reconciler.Adapter. A full replace.
func (a *LoadBalancerFrontendRuleAdapter) Update(ctx context.Context, r *lb.LoadBalancerFrontendRule) error {
	if err := a.parent(ctx, r); err != nil {
		return err
	}
	if _, err := a.API.GetLoadBalancerFrontendRule(ctx, &request.GetLoadBalancerFrontendRuleRequest{
		ServiceUUID: r.Status.ServiceUUID, FrontendName: r.Status.FrontendName, Name: r.ExternalName(),
	}); upcloudapi.IsNotFound(err) {
		return reconciler.ErrPending
	} else if err != nil {
		return fmt.Errorf("get load balancer frontend rule: %w", err)
	}
	_, err := a.API.ReplaceLoadBalancerFrontendRule(ctx, &request.ReplaceLoadBalancerFrontendRuleRequest{
		ServiceUUID:  r.Status.ServiceUUID,
		FrontendName: r.Status.FrontendName,
		Name:         r.ExternalName(),
		Rule:         toRule(r),
	})
	if err != nil {
		return fmt.Errorf("replace load balancer frontend rule: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A 404 counts as deleted.
func (a *LoadBalancerFrontendRuleAdapter) Delete(ctx context.Context, r *lb.LoadBalancerFrontendRule) error {
	if r.Status.ServiceUUID == "" || r.Status.FrontendName == "" || r.Status.Name == "" {
		return nil
	}
	err := a.API.DeleteLoadBalancerFrontendRule(ctx, &request.DeleteLoadBalancerFrontendRuleRequest{
		ServiceUUID: r.Status.ServiceUUID, FrontendName: r.Status.FrontendName, Name: r.Status.Name,
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	default:
		return fmt.Errorf("delete load balancer frontend rule: %w", err)
	}
}

// ruleUpToDate compares the observed rule to the desired (request) rule. Both
// carry the same upcloud matcher/action shapes, so we compare field by field.
func ruleUpToDate(observed *upcloud.LoadBalancerFrontendRule, desired request.LoadBalancerFrontendRule) bool {
	if observed.Name != desired.Name || observed.Priority != desired.Priority || observed.MatchingCondition != desired.MatchingCondition {
		return false
	}
	if !reflect.DeepEqual(observed.Matchers, desired.Matchers) {
		return false
	}
	if !reflect.DeepEqual(observed.Actions, desired.Actions) {
		return false
	}
	return true
}
