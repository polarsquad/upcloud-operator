package network

// Shared fixtures for the network/gateway adapter tests. Repeated literals are
// named so goconst does not flag them.
const (
	testZone           = "fi-hel1"
	testNat            = "nat"
	testVPN            = "vpn"
	testDefaultNS      = "default"
	testGWName         = "gw1"
	testConnName       = "conn1"
	testGWUID          = "gw-uid"
	testSmallPlan      = "small"
	testCIDR           = "10.0.0.0/24"
	testPskKey         = "psk"
	testReadyMsg       = "Available"
	testLocalAddrName  = "vpn"
	testStaticRoute    = "static"
	testReadyType      = "Ready"
	testGatewayUUID    = "gw-1"
	testConnectionUUID = "conn-1"
	testRemoteAddr     = "203.0.113.10"
	testConnUID        = "conn-uid"
	testIPFamily       = "IPv4"
	testIPAccess       = "public"
	testPeeringState   = "active"
	testNetUID         = "net-uid"
)
