package upcloudapi

import (
	"fmt"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/client"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/service"
)

// UserAgent identifies the operator in UpCloud request logs.
const UserAgent = "upcloud-operator"

// NewServiceFromEnv builds the UpCloud API client from UPCLOUD_TOKEN or
// UPCLOUD_USERNAME/UPCLOUD_PASSWORD. Exactly one method must be set.
func NewServiceFromEnv() (*service.Service, error) {
	c, err := client.NewFromEnv(client.WithTimeout(60 * time.Second))
	if err != nil {
		return nil, fmt.Errorf("upcloud credentials: %w (set UPCLOUD_TOKEN or UPCLOUD_USERNAME and UPCLOUD_PASSWORD)", err)
	}
	c.UserAgent = UserAgent
	return service.New(c), nil
}
