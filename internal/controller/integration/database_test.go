package integration_test

import (
	"context"
	"testing"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	. "github.com/onsi/gomega"

	"github.com/polarsquad/upcloud-operator/api/common"
	databasev1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/controller/database"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

func TestParentFirstDatabaseUserConvergesAfterCascade(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	c, api := pairClient(t), fake.NewDatabaseAPI()
	service := &databasev1.ManagedDatabase{
		ObjectMeta: pairMeta("parent-database"),
		Spec:       databasev1.ManagedDatabaseSpec{Type: "pg", Plan: "1x1xCPU-2GB", Zone: "fi-hel1"},
	}
	user := &databasev1.ManagedDatabaseUser{
		ObjectMeta: pairMeta("child-user"),
		Spec:       databasev1.ManagedDatabaseUserSpec{ServiceRef: common.LocalObjectReference{Name: service.Name}},
	}
	d := &reconciler.Reconciler[*databasev1.ManagedDatabase]{
		Client: c, Adapter: &database.ManagedDatabaseAdapter{Client: c, API: api},
		New:       func() *databasev1.ManagedDatabase { return &databasev1.ManagedDatabase{} },
		Finalizer: database.FinalizerManagedDatabase, PendingRequeue: retryDelay,
	}
	u := &reconciler.Reconciler[*databasev1.ManagedDatabaseUser]{
		Client: c, Adapter: &database.ManagedDatabaseUserAdapter{Client: c, API: api},
		New:       func() *databasev1.ManagedDatabaseUser { return &databasev1.ManagedDatabaseUser{} },
		Finalizer: database.FinalizerManagedDatabaseUser, PendingRequeue: retryDelay,
	}
	createReady(t, d, service)
	createReady(t, u, user)
	serviceID, username := service.Status.UUID, user.Status.Username
	g.Expect(user.Status.ServiceUUID).To(Equal(serviceID))
	g.Expect(user.Status.Type).To(Equal(string(upcloud.ManagedDatabaseUserTypeNormal)))

	requestDeletion(t, c, service)
	requirePending(t, d, service)
	requireLive(t, c, user)
	cloud, err := api.GetManagedDatabase(ctx, &request.GetManagedDatabaseRequest{UUID: serviceID})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cloud.State).To(Equal(upcloud.ManagedDatabaseStateDeleteService))
	g.Expect(upcloudapi.HasUID(cloud.Labels, service.UID)).To(BeTrue())
	cloudUser, err := api.GetManagedDatabaseUser(ctx, &request.GetManagedDatabaseUserRequest{ServiceUUID: serviceID, Username: username})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cloudUser.Username).To(Equal(username))

	requestDeletion(t, c, user)
	// Controlled transient-409 retry, NOT evidence that users block database
	// deletion or vice versa. This suite has no manager/background goroutine,
	// so the one-shot fault cannot race another fake API caller.
	api.FailNext = fake.Conflict("controlled transient user delete")
	requirePending(t, u, user)
	_, err = api.GetManagedDatabaseUser(ctx, &request.GetManagedDatabaseUserRequest{ServiceUUID: serviceID, Username: username})
	g.Expect(err).NotTo(HaveOccurred(), "the controlled failure must not remove the user")

	// Unlike MOS, a database does not wait for its users. The real parent
	// adapter's retry completes the fake's async delete, cascading its users.
	requireGone(t, d, service)
	_, err = api.GetManagedDatabase(ctx, &request.GetManagedDatabaseRequest{UUID: serviceID})
	g.Expect(upcloudapi.IsNotFound(err)).To(BeTrue())
	_, err = api.GetManagedDatabaseUser(ctx, &request.GetManagedDatabaseUserRequest{ServiceUUID: serviceID, Username: username})
	g.Expect(upcloudapi.IsNotFound(err)).To(BeTrue())

	// The user finalizer must use its saved service ID and accept the 404,
	// without trying to resolve a Ready parent that no longer exists.
	g.Expect(user.Status.ServiceUUID).To(Equal(serviceID))
	requireGone(t, u, user)
	g.Expect(deleteCalls(api.Calls)).To(Equal([]string{
		"DeleteManagedDatabase", "DeleteManagedDatabaseUser", "DeleteManagedDatabase", "DeleteManagedDatabaseUser",
	}))
}
