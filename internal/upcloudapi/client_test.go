package upcloudapi_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

func TestUserAgentIdentifiesUCK(t *testing.T) {
	g := NewWithT(t)
	g.Expect(upcloudapi.UserAgent).To(Equal("uck"))
}
