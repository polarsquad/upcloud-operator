package reconciler

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConditionReady is the single condition type every kind reports.
const ConditionReady = "Ready"

// Reasons for ConditionReady.
const (
	ReasonAvailable            = "Available"
	ReasonCreating             = "Creating"
	ReasonUpdating             = "Updating"
	ReasonPending              = "Pending"
	ReasonDeleting             = "Deleting"
	ReasonWaitingForDependency = "WaitingForDependency"
	ReasonObserveFailed        = "ObserveFailed"
	ReasonCreateFailed         = "CreateFailed"
	ReasonUpdateFailed         = "UpdateFailed"
	ReasonDeleteFailed         = "DeleteFailed"
)

// SetReady records the Ready condition with the object's current generation.
func SetReady(obj Object, status metav1.ConditionStatus, reason, message string) {
	conds := obj.GetConditions()
	meta.SetStatusCondition(&conds, metav1.Condition{
		Type:               ConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: obj.GetGeneration(),
	})
	obj.SetConditions(conds)
}

// IsReady reports whether the Ready condition is True.
func IsReady(obj Object) bool {
	return meta.IsStatusConditionTrue(obj.GetConditions(), ConditionReady)
}
