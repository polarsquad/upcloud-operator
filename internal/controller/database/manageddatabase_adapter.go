package database

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	databasev1alpha1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/k8s"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/resolve"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// ManagedDatabaseAdapter maps ManagedDatabase onto the UpCloud Managed
// Database API.
type ManagedDatabaseAdapter struct {
	API    upcloudapi.DatabaseAPI
	Client client.Client
}

var _ reconciler.Adapter[*databasev1alpha1.ManagedDatabase] = (*ManagedDatabaseAdapter)(nil)

// updateOKStates are the states in which the service accepts modification
// or start/shutdown requests.
var updateOKStates = map[upcloud.ManagedDatabaseState]bool{
	upcloud.ManagedDatabaseStateRunning: true,
	upcloud.ManagedDatabaseStateStopped: true,
	upcloud.ManagedDatabaseStateError:   true,
}

// deleteOKStates extends updateOKStates with delete-service, where a second
// DeleteManagedDatabase call finishes the deletion.
var deleteOKStates = map[upcloud.ManagedDatabaseState]bool{
	upcloud.ManagedDatabaseStateRunning:       true,
	upcloud.ManagedDatabaseStateStopped:       true,
	upcloud.ManagedDatabaseStateError:         true,
	upcloud.ManagedDatabaseStateDeleteService: true,
}

func (a *ManagedDatabaseAdapter) lookup(ctx context.Context, d *databasev1alpha1.ManagedDatabase) (*upcloud.ManagedDatabase, error) {
	if d.Status.UUID != "" {
		db, err := a.API.GetManagedDatabase(ctx, &request.GetManagedDatabaseRequest{UUID: d.Status.UUID})
		if upcloudapi.IsNotFound(err) {
			return nil, nil
		}
		return db, err
	}
	list, err := a.API.GetAllManagedDatabases(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if upcloudapi.HasUID(list[i].Labels, d.UID) {
			return &list[i], nil
		}
	}
	return nil, nil
}

// desiredNetworks resolves the spec network attachments, turning networkRef
// entries into the referenced Network CR's UUID.
func (a *ManagedDatabaseAdapter) desiredNetworks(ctx context.Context, d *databasev1alpha1.ManagedDatabase) ([]upcloud.ManagedDatabaseNetwork, error) {
	out := make([]upcloud.ManagedDatabaseNetwork, 0, len(d.Spec.Networks))
	for _, att := range d.Spec.Networks {
		uuid, err := resolve.NetworkUUID(ctx, a.Client, d.Namespace, att)
		if err != nil {
			return nil, err
		}
		n := upcloud.ManagedDatabaseNetwork{Name: att.Name, Type: att.Type, Family: att.Family}
		if uuid != "" {
			n.UUID = &uuid
		}
		out = append(out, n)
	}
	return out, nil
}

// propertiesRequest parses spec.properties into the request property map.
func propertiesRequest(raw *apiextensionsv1.JSON) (request.ManagedDatabasePropertiesRequest, error) {
	if raw == nil || len(raw.Raw) == 0 {
		return nil, nil
	}
	var m request.ManagedDatabasePropertiesRequest
	if err := json.Unmarshal(raw.Raw, &m); err != nil {
		return nil, fmt.Errorf("spec.properties is not a JSON object: %w", err)
	}
	return m, nil
}

// Observe implements reconciler.Adapter.
func (a *ManagedDatabaseAdapter) Observe(ctx context.Context, d *databasev1alpha1.ManagedDatabase) (reconciler.Observation, error) {
	db, err := a.lookup(ctx, d)
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get managed database: %w", err)
	}
	if db == nil {
		return reconciler.Observation{}, nil
	}
	d.Status.UUID = db.UUID
	a.syncStatus(d, db)

	networks, err := a.desiredNetworks(ctx, d)
	if err != nil {
		return reconciler.Observation{}, err
	}
	ready := stateReady(db, d)
	if ready && db.ServiceURIParams.Host != "" {
		if err := a.writeConnectionSecret(ctx, d, db); err != nil {
			return reconciler.Observation{}, err
		}
	}
	return reconciler.Observation{
		Exists:   true,
		UpToDate: specMatches(db, d, networks) && db.Powered == d.DesiredPowered(),
		Ready:    ready,
		Message:  observationMessage(db),
	}, nil
}

// Create implements reconciler.Adapter.
func (a *ManagedDatabaseAdapter) Create(ctx context.Context, d *databasev1alpha1.ManagedDatabase) error {
	networks, err := a.desiredNetworks(ctx, d)
	if err != nil {
		return err
	}
	props, err := propertiesRequest(d.Spec.Properties)
	if err != nil {
		return err
	}
	req := &request.CreateManagedDatabaseRequest{
		Type:                   upcloud.ManagedDatabaseServiceType(d.Spec.Type),
		Plan:                   d.Spec.Plan,
		Zone:                   d.Spec.Zone,
		Title:                  d.ExternalTitle(),
		HostNamePrefix:         d.ExternalHostNamePrefix(),
		AdditionalDiskSpaceGiB: d.Spec.AdditionalDiskSpaceGiB,
		TerminationProtection:  &d.Spec.TerminationProtection,
		Labels:                 upcloudapi.DesiredLabels(d, d.Spec.Labels),
		Networks:               networks,
		Properties:             props,
	}
	if d.Spec.Maintenance != nil {
		req.Maintenance = request.ManagedDatabaseMaintenanceTimeRequest{
			DayOfWeek: d.Spec.Maintenance.DayOfWeek,
			Time:      d.Spec.Maintenance.Time,
		}
	}
	created, err := a.API.CreateManagedDatabase(ctx, req)
	if err != nil {
		return fmt.Errorf("create managed database: %w", err)
	}
	d.Status.UUID = created.UUID
	return nil
}

// Update implements reconciler.Adapter. Powered drift is applied with
// Start/Shutdown first; everything else goes through one modify call.
func (a *ManagedDatabaseAdapter) Update(ctx context.Context, d *databasev1alpha1.ManagedDatabase) error {
	db, err := a.lookup(ctx, d)
	if err != nil {
		return fmt.Errorf("get managed database: %w", err)
	}
	if db == nil {
		return fmt.Errorf("update managed database: %w", reconciler.ErrPending)
	}
	if !updateOKStates[db.State] {
		return reconciler.ErrPending
	}

	switch {
	case db.Powered && !d.DesiredPowered():
		if _, err := a.API.ShutdownManagedDatabase(ctx, &request.ShutdownManagedDatabaseRequest{UUID: db.UUID}); err != nil {
			return fmt.Errorf("shutdown managed database: %w", err)
		}
	case !db.Powered && d.DesiredPowered():
		if _, err := a.API.StartManagedDatabase(ctx, &request.StartManagedDatabaseRequest{UUID: db.UUID}); err != nil {
			return fmt.Errorf("start managed database: %w", err)
		}
	}

	networks, err := a.desiredNetworks(ctx, d)
	if err != nil {
		return err
	}
	if specMatches(db, d, networks) {
		return nil
	}
	props, err := propertiesRequest(d.Spec.Properties)
	if err != nil {
		return err
	}
	labels := upcloudapi.DesiredLabels(d, d.Spec.Labels)
	modify := &request.ModifyManagedDatabaseRequest{
		UUID:                   db.UUID,
		Plan:                   d.Spec.Plan,
		Title:                  d.ExternalTitle(),
		AdditionalDiskSpaceGiB: &d.Spec.AdditionalDiskSpaceGiB,
		TerminationProtection:  &d.Spec.TerminationProtection,
		Labels:                 &labels,
		Networks:               &networks,
		Properties:             props,
	}
	if d.Spec.Maintenance != nil {
		modify.Maintenance = request.ManagedDatabaseMaintenanceTimeRequest{
			DayOfWeek: d.Spec.Maintenance.DayOfWeek,
			Time:      d.Spec.Maintenance.Time,
		}
	}
	if _, err := a.API.ModifyManagedDatabase(ctx, modify); err != nil {
		return fmt.Errorf("modify managed database: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A successful delete request is
// reported as pending: the service stays visible for a while, and a later
// pass sees it gone (404) and returns nil.
func (a *ManagedDatabaseAdapter) Delete(ctx context.Context, d *databasev1alpha1.ManagedDatabase) error {
	if d.Status.UUID == "" {
		return nil
	}
	db, err := a.API.GetManagedDatabase(ctx, &request.GetManagedDatabaseRequest{UUID: d.Status.UUID})
	if upcloudapi.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get managed database: %w", err)
	}
	if !deleteOKStates[db.State] {
		return reconciler.ErrPending
	}
	err = a.API.DeleteManagedDatabase(ctx, &request.DeleteManagedDatabaseRequest{UUID: db.UUID})
	switch {
	case err == nil:
		return reconciler.ErrPending
	case upcloudapi.IsNotFound(err):
		return nil
	default:
		return fmt.Errorf("delete managed database: %w", err)
	}
}

// stateReady reports running, or stopped when the spec asks for stopped.
func stateReady(db *upcloud.ManagedDatabase, d *databasev1alpha1.ManagedDatabase) bool {
	switch db.State {
	case upcloud.ManagedDatabaseStateRunning:
		return true
	case upcloud.ManagedDatabaseStateStopped:
		return !d.DesiredPowered()
	default:
		return false
	}
}

func observationMessage(db *upcloud.ManagedDatabase) string {
	msg := "state " + string(db.State)
	if len(db.StateError) > 0 {
		keys := make([]upcloud.ManagedDatabaseState, 0, len(db.StateError))
		for k := range db.StateError {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		msg += ": " + db.StateError[keys[0]]
	}
	return msg
}

func (a *ManagedDatabaseAdapter) syncStatus(d *databasev1alpha1.ManagedDatabase, db *upcloud.ManagedDatabase) {
	d.Status.State = string(db.State)
	d.Status.PrimaryHost = db.ServiceURIParams.Host
	d.Status.PrimaryPort = atoi(db.ServiceURIParams.Port)
	d.Status.ServiceURIHost = db.ServiceURIParams.Host
	d.Status.Version = versionOf(db)
	d.Status.NodeCount = db.NodeCount
	d.Status.Powered = db.Powered
}

func versionOf(db *upcloud.ManagedDatabase) string {
	if v, ok := db.Properties[upcloud.ManagedDatabasePropertyKey("version")].(string); ok {
		return v
	}
	return ""
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// specMatches compares the mutable spec fields against the service.
// Powered is checked by the caller, and maintenance is only compared when the
// spec sets it.
func specMatches(db *upcloud.ManagedDatabase, d *databasev1alpha1.ManagedDatabase, networks []upcloud.ManagedDatabaseNetwork) bool {
	if db.Plan != d.Spec.Plan {
		return false
	}
	if db.Title != d.ExternalTitle() {
		return false
	}
	if db.AdditionalDiskSpaceGiB != d.Spec.AdditionalDiskSpaceGiB {
		return false
	}
	if db.TerminationProtection != d.Spec.TerminationProtection {
		return false
	}
	if d.Spec.Maintenance != nil &&
		(db.Maintenance.DayOfWeek != d.Spec.Maintenance.DayOfWeek || db.Maintenance.Time != d.Spec.Maintenance.Time) {
		return false
	}
	if !upcloudapi.LabelsEqual(db.Labels, upcloudapi.DesiredLabels(d, d.Spec.Labels)) {
		return false
	}
	if !networksEqual(db.Networks, networks) {
		return false
	}
	return propertiesUpToDate(db.Properties, d.Spec.Properties)
}

// propertiesUpToDate requires every spec.properties key to be present in the
// observed properties with an equal JSON value. Keys UpCloud fills in itself
// are ignored.
func propertiesUpToDate(observed upcloud.ManagedDatabaseProperties, raw *apiextensionsv1.JSON) bool {
	if raw == nil || len(raw.Raw) == 0 {
		return true
	}
	var specProps map[string]any
	if err := json.Unmarshal(raw.Raw, &specProps); err != nil {
		return true
	}
	for k, v := range specProps {
		ob, ok := observed[upcloud.ManagedDatabasePropertyKey(k)]
		if !ok {
			return false
		}
		want, _ := json.Marshal(v)
		got, _ := json.Marshal(ob)
		if string(want) != string(got) {
			return false
		}
	}
	return true
}

func networkKey(n upcloud.ManagedDatabaseNetwork) string {
	uuid := ""
	if n.UUID != nil {
		uuid = *n.UUID
	}
	return n.Name + "|" + n.Type + "|" + n.Family + "|" + uuid
}

func networksEqual(a, b []upcloud.ManagedDatabaseNetwork) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]bool, len(a))
	for _, n := range a {
		set[networkKey(n)] = true
	}
	for _, n := range b {
		if !set[networkKey(n)] {
			return false
		}
	}
	return true
}

// writeConnectionSecret writes the primary connection details as an owned
// Secret in the CR's namespace.
func (a *ManagedDatabaseAdapter) writeConnectionSecret(ctx context.Context, d *databasev1alpha1.ManagedDatabase, db *upcloud.ManagedDatabase) error {
	p := db.ServiceURIParams
	data := map[string][]byte{
		SecretKeyURI:      []byte(db.ServiceURI),
		SecretKeyHost:     []byte(p.Host),
		SecretKeyPort:     []byte(p.Port),
		SecretKeyUser:     []byte(p.User),
		SecretKeyPassword: []byte(p.Password),
		SecretKeyDBName:   []byte(p.DatabaseName),
		SecretKeySSLMode:  []byte(p.SSLMode),
	}
	return k8s.WriteOwnedSecret(ctx, a.Client, d, d.ConnectionSecretName(), data)
}
