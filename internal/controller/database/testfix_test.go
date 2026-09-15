package database

// Shared fixtures for the database adapter tests. Repeated literals are
// named so goconst does not flag them.
const (
	testNS      = "ns"
	testPlan    = "3x25"
	testZone    = "fi-hel1"
	mdbParent   = "mdb-parent"
	parentName  = "parent"
	upadminUser = "upadmin"
	pwValue     = "value"
	daveCreds   = "dave-credentials"
	envtestNS   = "default"
	envtestSvc  = "mdb"
	testLDBName = "analytics"
	md1ConnName = "md1-connection"
)
