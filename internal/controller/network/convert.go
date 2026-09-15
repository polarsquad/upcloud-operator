package network

import (
	"slices"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"

	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
)

func toStaticRoutes(in []networkv1alpha1.StaticRoute) []upcloud.StaticRoute {
	out := make([]upcloud.StaticRoute, 0, len(in))
	for _, r := range in {
		out = append(out, upcloud.StaticRoute{Name: r.Name, Route: r.Route, Nexthop: r.Nexthop, Type: upcloud.RouterStaticRouteTypeUser})
	}
	return out
}

// userRoutes drops UpCloud-managed "service" routes so drift detection only
// compares what the user controls.
func userRoutes(in []upcloud.StaticRoute) []upcloud.StaticRoute {
	out := make([]upcloud.StaticRoute, 0, len(in))
	for _, r := range in {
		if r.Type == upcloud.RouterStaticRouteTypeUser || r.Type == "" {
			out = append(out, upcloud.StaticRoute{Name: r.Name, Route: r.Route, Nexthop: r.Nexthop, Type: upcloud.RouterStaticRouteTypeUser})
		}
	}
	return out
}

func routesEqual(a, b []upcloud.StaticRoute) bool {
	key := func(r upcloud.StaticRoute) string { return r.Name + "|" + r.Route + "|" + r.Nexthop }
	as, bs := slices.Clone(a), slices.Clone(b)
	slices.SortFunc(as, func(x, y upcloud.StaticRoute) int { return compareStrings(key(x), key(y)) })
	slices.SortFunc(bs, func(x, y upcloud.StaticRoute) int { return compareStrings(key(x), key(y)) })
	return slices.EqualFunc(as, bs, func(x, y upcloud.StaticRoute) bool { return key(x) == key(y) })
}

func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
