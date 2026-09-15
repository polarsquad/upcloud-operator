package upcloudapi_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/client"
	. "github.com/onsi/gomega"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

func TestIsNotFoundMatchesProblemAndClientErrorsAndWrapping(t *testing.T) {
	g := NewWithT(t)
	p := &upcloud.Problem{Status: http.StatusNotFound, Title: "gone"}
	g.Expect(upcloudapi.IsNotFound(p)).To(BeTrue())
	g.Expect(upcloudapi.IsNotFound(fmt.Errorf("wrapped: %w", p))).To(BeTrue())
	g.Expect(upcloudapi.IsNotFound(&client.Error{ErrorCode: 404})).To(BeTrue())
	g.Expect(upcloudapi.IsNotFound(&upcloud.Problem{Status: 409})).To(BeFalse())
	g.Expect(upcloudapi.IsNotFound(errors.New("plain"))).To(BeFalse())
}

func TestIsConflict(t *testing.T) {
	g := NewWithT(t)
	g.Expect(upcloudapi.IsConflict(&upcloud.Problem{Status: http.StatusConflict})).To(BeTrue())
	g.Expect(upcloudapi.IsConflict(&upcloud.Problem{Status: 400})).To(BeFalse())
}
