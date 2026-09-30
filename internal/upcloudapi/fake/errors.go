// Package fake provides in-memory implementations of the upcloudapi interfaces.
package fake

import (
	"net/http"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
)

// NotFound is the error the real API returns for unknown identifiers.
func NotFound(what string) error {
	return &upcloud.Problem{Status: http.StatusNotFound, Title: what + " not found"}
}

// Conflict mimics a 409 from the API.
func Conflict(what string) error {
	return &upcloud.Problem{Status: http.StatusConflict, Title: what + " already exists"}
}

// Invalid mimics a 400 validation error from the API.
func Invalid(msg string) error {
	return &upcloud.Problem{Status: http.StatusBadRequest, Title: msg}
}
