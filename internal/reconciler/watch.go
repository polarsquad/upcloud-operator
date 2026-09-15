package reconciler

import (
	"context"
	"slices"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// DependentsOf returns a MapFunc that enqueues every item of list (same
// namespace as the changed object) whose refNames include the changed
// object's name. Use it to re-reconcile children when a parent changes.
func DependentsOf[T client.Object, L client.ObjectList](
	c client.Client, newList func() L, items func(L) []T, refNames func(T) []string,
) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []ctrl.Request {
		list := newList()
		if err := c.List(ctx, list, client.InNamespace(obj.GetNamespace())); err != nil {
			logf.FromContext(ctx).Error(err, "list dependents")
			return nil
		}
		var out []ctrl.Request
		for _, it := range items(list) {
			if slices.Contains(refNames(it), obj.GetName()) {
				out = append(out, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: it.GetNamespace(), Name: it.GetName()}})
			}
		}
		return out
	}
}
