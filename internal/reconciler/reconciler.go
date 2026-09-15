// Package reconciler holds the generic reconcile loop shared by every kind.
package reconciler

import (
	"context"
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/polarsquad/upcloud-operator/api/common"
)

var (
	// ErrDependencyNotReady is returned by adapters when a referenced CR or
	// Secret does not exist yet or is not Ready. Reconcile requeues quietly.
	ErrDependencyNotReady = errors.New("dependency not ready")
	// ErrPending is returned by Update or Delete when the external resource
	// is in a transitional state and the operation must be retried later.
	ErrPending = errors.New("external operation in progress")
)

// Object is what every managed CRD type implements.
type Object interface {
	client.Object
	GetConditions() []metav1.Condition
	SetConditions([]metav1.Condition)
	GetDeletionPolicy() common.DeletionPolicy
	// GetExternalID is the UpCloud identifier (UUID or name), "" until created.
	GetExternalID() string
}

// Observation is the adapter's view of the external resource.
type Observation struct {
	Exists   bool
	UpToDate bool
	Ready    bool
	Message  string
}

// Options are the manager-level tuning knobs shared by every controller.
// Zero values fall back to the documented defaults in Reconciler.defaults.
type Options struct {
	// SteadyRequeue is the drift-detection interval for Ready objects
	// (default 5m). Tune with the --steady-requeue manager flag.
	SteadyRequeue time.Duration
}

// Adapter maps one kind onto the UpCloud API.
type Adapter[T Object] interface {
	Observe(ctx context.Context, obj T) (Observation, error)
	Create(ctx context.Context, obj T) error
	Update(ctx context.Context, obj T) error
	Delete(ctx context.Context, obj T) error
}

// Reconciler drives an Adapter.
type Reconciler[T Object] struct {
	client.Client
	Adapter   Adapter[T]
	New       func() T
	Finalizer string

	PendingRequeue    time.Duration // after Create/Update or while not Ready (default 15s)
	DependencyRequeue time.Duration // while ErrDependencyNotReady (default 15s)
	SteadyRequeue     time.Duration // drift detection interval when Ready (default 5m)
}

func (r *Reconciler[T]) defaults() {
	if r.PendingRequeue == 0 {
		r.PendingRequeue = 15 * time.Second
	}
	if r.DependencyRequeue == 0 {
		r.DependencyRequeue = 15 * time.Second
	}
	if r.SteadyRequeue == 0 {
		r.SteadyRequeue = 5 * time.Minute
	}
}

// ApplyOptions sets the manager-level tuning knobs on the reconciler.
// Zero-valued options are ignored so the defaults stay in one place.
func (r *Reconciler[T]) ApplyOptions(o Options) {
	if o.SteadyRequeue > 0 {
		r.SteadyRequeue = o.SteadyRequeue
	}
}

// Reconcile implements reconcile.Reconciler.
func (r *Reconciler[T]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.defaults()
	obj := r.New()
	if err := r.Get(ctx, req.NamespacedName, obj); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		return r.reconcileDelete(ctx, obj)
	}

	if !controllerutil.ContainsFinalizer(obj, r.Finalizer) {
		controllerutil.AddFinalizer(obj, r.Finalizer)
		return ctrl.Result{}, r.Update(ctx, obj)
	}

	return r.reconcileNormal(ctx, obj)
}

func (r *Reconciler[T]) reconcileNormal(ctx context.Context, obj T) (ctrl.Result, error) {
	before := obj.DeepCopyObject().(client.Object)

	obs, err := r.Adapter.Observe(ctx, obj)
	if err != nil {
		if errors.Is(err, ErrDependencyNotReady) {
			SetReady(obj, metav1.ConditionFalse, ReasonWaitingForDependency, err.Error())
			return ctrl.Result{RequeueAfter: r.DependencyRequeue}, r.patchStatus(ctx, obj, before)
		}
		SetReady(obj, metav1.ConditionFalse, ReasonObserveFailed, err.Error())
		return ctrl.Result{}, errors.Join(err, r.patchStatus(ctx, obj, before))
	}

	switch {
	case !obs.Exists:
		if err := r.Adapter.Create(ctx, obj); err != nil {
			return r.fail(ctx, obj, before, ReasonCreateFailed, err)
		}
		SetReady(obj, metav1.ConditionFalse, ReasonCreating, "create request accepted")
		return ctrl.Result{RequeueAfter: r.PendingRequeue}, r.patchStatus(ctx, obj, before)

	case !obs.UpToDate:
		err := r.Adapter.Update(ctx, obj)
		switch {
		case errors.Is(err, ErrPending):
			SetReady(obj, metav1.ConditionFalse, ReasonPending, err.Error())
		case errors.Is(err, ErrDependencyNotReady):
			SetReady(obj, metav1.ConditionFalse, ReasonWaitingForDependency, err.Error())
		case err != nil:
			return r.fail(ctx, obj, before, ReasonUpdateFailed, err)
		default:
			SetReady(obj, metav1.ConditionFalse, ReasonUpdating, "update request accepted")
		}
		return ctrl.Result{RequeueAfter: r.PendingRequeue}, r.patchStatus(ctx, obj, before)

	case !obs.Ready:
		SetReady(obj, metav1.ConditionFalse, ReasonPending, obs.Message)
		return ctrl.Result{RequeueAfter: r.PendingRequeue}, r.patchStatus(ctx, obj, before)

	default:
		SetReady(obj, metav1.ConditionTrue, ReasonAvailable, obs.Message)
		return ctrl.Result{RequeueAfter: r.SteadyRequeue}, r.patchStatus(ctx, obj, before)
	}
}

func (r *Reconciler[T]) reconcileDelete(ctx context.Context, obj T) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(obj, r.Finalizer) {
		return ctrl.Result{}, nil
	}
	before := obj.DeepCopyObject().(client.Object)

	if obj.GetDeletionPolicy() != common.DeletionPolicyOrphan {
		err := r.Adapter.Delete(ctx, obj)
		switch {
		case errors.Is(err, ErrPending):
			SetReady(obj, metav1.ConditionFalse, ReasonDeleting, err.Error())
			return ctrl.Result{RequeueAfter: r.PendingRequeue}, r.patchStatus(ctx, obj, before)
		case err != nil:
			return r.fail(ctx, obj, before, ReasonDeleteFailed, err)
		}
	}

	controllerutil.RemoveFinalizer(obj, r.Finalizer)
	return ctrl.Result{}, r.Update(ctx, obj)
}

func (r *Reconciler[T]) fail(ctx context.Context, obj T, before client.Object, reason string, err error) (ctrl.Result, error) {
	logf.FromContext(ctx).Error(err, "reconcile failed", "reason", reason)
	SetReady(obj, metav1.ConditionFalse, reason, err.Error())
	return ctrl.Result{}, errors.Join(err, r.patchStatus(ctx, obj, before))
}

func (r *Reconciler[T]) patchStatus(ctx context.Context, obj T, before client.Object) error {
	return r.Status().Patch(ctx, obj, client.MergeFrom(before))
}
