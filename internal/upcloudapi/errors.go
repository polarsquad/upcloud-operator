package upcloudapi

import (
	"errors"
	"net/http"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/client"
)

// StatusOf extracts the HTTP status from an UpCloud SDK error, 0 when unknown.
func StatusOf(err error) int {
	var p *upcloud.Problem
	if errors.As(err, &p) {
		return p.Status
	}
	var ce *client.Error
	if errors.As(err, &ce) {
		return ce.ErrorCode
	}
	return 0
}

// IsNotFound reports a 404 from the UpCloud API.
func IsNotFound(err error) bool { return StatusOf(err) == http.StatusNotFound }

// IsConflict reports a 409 from the UpCloud API (resource already exists).
func IsConflict(err error) bool { return StatusOf(err) == http.StatusConflict }

// TitleOf extracts the human-readable title from an UpCloud API error,
// returning the bare error string when the problem carries no title.
func TitleOf(err error) string {
	var p *upcloud.Problem
	if errors.As(err, &p) && p.Title != "" {
		return p.Title
	}
	return err.Error()
}
