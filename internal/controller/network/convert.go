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

func ptrDefault[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

// toIPNetworks maps CRD subnets to the UpCloud create/modify request shape,
// applying the CRD defaults for DHCP and family.
func toIPNetworks(in []networkv1alpha1.IPNetwork) upcloud.IPNetworkSlice {
	out := make(upcloud.IPNetworkSlice, 0, len(in))
	for _, p := range in {
		family := p.Family
		if family == "" {
			family = "IPv4"
		}
		out = append(out, upcloud.IPNetwork{
			Address:          p.Address,
			Family:           family,
			DHCP:             upcloud.FromBool(ptrDefault(p.DHCP, true)),
			DHCPDefaultRoute: upcloud.FromBool(ptrDefault(p.DHCPDefaultRoute, false)),
			DHCPDns:          p.DHCPDns,
			DHCPRoutes:       p.DHCPRoutes,
			Gateway:          p.Gateway,
		})
	}
	return out
}

func sortedByAddress(in upcloud.IPNetworkSlice) upcloud.IPNetworkSlice {
	out := slices.Clone(in)
	slices.SortFunc(out, func(a, b upcloud.IPNetwork) int { return compareStrings(a.Address, b.Address) })
	return out
}

// ipNetworksEqual compares subnets per address. Gateway is only compared
// when the desired gateway is non-empty: UpCloud fills a gateway when the
// user omits one, which must not count as drift.
func ipNetworksEqual(observed, desired upcloud.IPNetworkSlice) bool {
	o, d := sortedByAddress(observed), sortedByAddress(desired)
	if len(o) != len(d) {
		return false
	}
	for i := range o {
		if o[i].Address != d[i].Address || o[i].Family != d[i].Family ||
			o[i].DHCP.Bool() != d[i].DHCP.Bool() ||
			o[i].DHCPDefaultRoute.Bool() != d[i].DHCPDefaultRoute.Bool() ||
			!slices.Equal(o[i].DHCPDns, d[i].DHCPDns) ||
			!slices.Equal(o[i].DHCPRoutes, d[i].DHCPRoutes) {
			return false
		}
		if d[i].Gateway != "" && o[i].Gateway != d[i].Gateway {
			return false
		}
	}
	return true
}
