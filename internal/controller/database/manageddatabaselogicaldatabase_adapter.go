package database

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"sigs.k8s.io/controller-runtime/pkg/client"

	databasev1alpha1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/resolve"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// LogicalDatabaseAdapter maps ManagedDatabaseLogicalDatabase onto the
// UpCloud managed database logical database API. There is no update path:
// collation changes require deleting and recreating the resource, so
// UpToDate is always true once the logical database exists.
type LogicalDatabaseAdapter struct {
	API    upcloudapi.DatabaseAPI
	Client client.Client
}

var _ reconciler.Adapter[*databasev1alpha1.ManagedDatabaseLogicalDatabase] = (*LogicalDatabaseAdapter)(nil)

// parentUUID returns the service UUID, resolving the referenced
// ManagedDatabase (which must be Ready) on first use and storing the result
// in Status.
func (a *LogicalDatabaseAdapter) parentUUID(ctx context.Context, d *databasev1alpha1.ManagedDatabaseLogicalDatabase) (string, error) {
	if d.Status.ServiceUUID != "" {
		return d.Status.ServiceUUID, nil
	}
	uuid, err := resolve.Ready(ctx, a.Client, d.Namespace, d.Spec.ServiceRef.Name, &databasev1alpha1.ManagedDatabase{})
	if err != nil {
		return "", err
	}
	d.Status.ServiceUUID = uuid
	return uuid, nil
}

// Observe implements reconciler.Adapter. The SDK has no single GET for a
// logical database, so the list is filtered by name; a missing entry
// reports Exists: false.
func (a *LogicalDatabaseAdapter) Observe(ctx context.Context, d *databasev1alpha1.ManagedDatabaseLogicalDatabase) (reconciler.Observation, error) {
	svcUUID, err := a.parentUUID(ctx, d)
	if err != nil {
		return reconciler.Observation{}, err
	}
	list, err := a.API.GetManagedDatabaseLogicalDatabases(ctx, &request.GetManagedDatabaseLogicalDatabasesRequest{ServiceUUID: svcUUID})
	if upcloudapi.IsNotFound(err) {
		return reconciler.Observation{}, nil
	}
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get managed database logical databases: %w", err)
	}
	for _, db := range list {
		if db.Name == d.ExternalName() {
			d.Status.Name = db.Name
			return reconciler.Observation{Exists: true, UpToDate: true, Ready: true, Message: "logical database " + db.Name}, nil
		}
	}
	return reconciler.Observation{}, nil
}

// Create implements reconciler.Adapter.
func (a *LogicalDatabaseAdapter) Create(ctx context.Context, d *databasev1alpha1.ManagedDatabaseLogicalDatabase) error {
	svcUUID, err := a.parentUUID(ctx, d)
	if err != nil {
		return err
	}
	created, err := a.API.CreateManagedDatabaseLogicalDatabase(ctx, &request.CreateManagedDatabaseLogicalDatabaseRequest{
		ServiceUUID: svcUUID,
		Name:        d.ExternalName(),
		LCCollate:   d.Spec.LCCollate,
		LCCType:     d.Spec.LCCType,
	})
	if err != nil {
		return fmt.Errorf("create managed database logical database: %w", err)
	}
	d.Status.ServiceUUID = svcUUID
	d.Status.Name = created.Name
	return nil
}

// Update implements reconciler.Adapter. Collation is create-only, so an
// existing logical database is always up to date.
func (a *LogicalDatabaseAdapter) Update(ctx context.Context, d *databasev1alpha1.ManagedDatabaseLogicalDatabase) error {
	return nil
}

// Delete implements reconciler.Adapter. Idempotent: a gone logical database
// or a gone service both count as deleted.
func (a *LogicalDatabaseAdapter) Delete(ctx context.Context, d *databasev1alpha1.ManagedDatabaseLogicalDatabase) error {
	if d.Status.ServiceUUID == "" {
		return nil
	}
	err := a.API.DeleteManagedDatabaseLogicalDatabase(ctx, &request.DeleteManagedDatabaseLogicalDatabaseRequest{
		ServiceUUID: d.Status.ServiceUUID,
		Name:        d.ExternalName(),
	})
	switch {
	case err == nil, upcloudapi.IsNotFound(err):
		return nil
	default:
		return fmt.Errorf("delete managed database logical database: %w", err)
	}
}
