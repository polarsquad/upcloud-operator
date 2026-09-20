package database

import (
	"errors"
	"net/http"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
)

// Shared fixtures for the database adapter tests. Repeated literals are
// named so goconst does not flag them.
const (
	testNS         = "ns"
	testPlan       = "3x25"
	testZone       = "fi-hel1"
	mdbParent      = "mdb-parent"
	parentName     = "parent"
	upadminUser    = "upadmin"
	pwValue        = "value"
	daveCreds      = "dave-credentials"
	envtestNS      = "default"
	envtestSvc     = "mdb"
	testLDBName    = "analytics"
	md1ConnName    = "md1-connection"
	deleteUser     = "delete-user"
	getMDCall      = "GetManagedDatabase"
	deleteMDCall   = "DeleteManagedDatabase"
	deleteLDBCall  = "DeleteManagedDatabaseLogicalDatabase"
	deleteUserCall = "DeleteManagedDatabaseUser"
)

func deleteErrorCases() map[string]error {
	return map[string]error{
		"unauthorized": &upcloud.Problem{Status: http.StatusUnauthorized, Title: "not authenticated"},
		"forbidden":    &upcloud.Problem{Status: http.StatusForbidden, Title: "not authorized"},
		"server_error": &upcloud.Problem{Status: http.StatusInternalServerError, Title: "service unavailable"},
		"transport":    errors.New("connection interrupted"),
	}
}
