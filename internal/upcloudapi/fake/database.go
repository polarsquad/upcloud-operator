package fake

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

var _ upcloudapi.DatabaseAPI = (*DatabaseAPI)(nil)

// DatabaseAPI is an in-memory upcloudapi.DatabaseAPI.
//
// Users are stored inside Databases[uuid].Users. Logical databases are stored
// in the LogicalDBs side map keyed by service UUID.
type DatabaseAPI struct {
	mu        sync.Mutex
	seq       int
	Databases  map[string]*upcloud.ManagedDatabase
	LogicalDBs map[string][]upcloud.ManagedDatabaseLogicalDatabase
	Calls      []string // method names in call order
	// StateOverride, when set, is reported by the GET endpoints in place of
	// the stored state, to simulate transitional states such as rebuilding.
	StateOverride upcloud.ManagedDatabaseState
	// FailNext, when set, is returned by the next mutating call and cleared.
	FailNext error
}

// NewDatabaseAPI returns an empty fake.
func NewDatabaseAPI() *DatabaseAPI {
	return &DatabaseAPI{
		Databases:  map[string]*upcloud.ManagedDatabase{},
		LogicalDBs: map[string][]upcloud.ManagedDatabaseLogicalDatabase{},
	}
}

// record appends the call to Calls. Callers must hold f.mu.
func (f *DatabaseAPI) record(name string) error {
	f.Calls = append(f.Calls, name)
	if f.FailNext != nil {
		err := f.FailNext
		f.FailNext = nil
		return err
	}
	return nil
}

func (f *DatabaseAPI) nextUUID(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%04d", prefix, f.seq)
}

func (f *DatabaseAPI) serviceURIParams(uuid string) upcloud.ManagedDatabaseServiceURIParams {
	return upcloud.ManagedDatabaseServiceURIParams{
		Host:         uuid + ".db.upclouddatabases.com",
		Port:         "11569",
		User:         "upadmin",
		Password:     "fake-pw",
		DatabaseName: "defaultdb",
		SSLMode:      "require",
	}
}

func (f *DatabaseAPI) composedURI(p upcloud.ManagedDatabaseServiceURIParams) string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
		p.User, p.Password, p.Host, p.Port, p.DatabaseName, p.SSLMode)
}

// copy applies StateOverride to a copy of db, so tests can simulate a
// transitional state without mutating the stored resource.
func (f *DatabaseAPI) copy(db *upcloud.ManagedDatabase) *upcloud.ManagedDatabase {
	cp := *db
	if f.StateOverride != "" {
		cp.State = f.StateOverride
	}
	return &cp
}

// CreateManagedDatabase implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) CreateManagedDatabase(_ context.Context, r *request.CreateManagedDatabaseRequest) (*upcloud.ManagedDatabase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateManagedDatabase"); err != nil {
		return nil, err
	}
	uuid := f.nextUUID("mdb")
	params := f.serviceURIParams(uuid)
	db := &upcloud.ManagedDatabase{
		UUID:                   uuid,
		Name:                   r.HostNamePrefix,
		Title:                  r.Title,
		Type:                   r.Type,
		Plan:                   r.Plan,
		Zone:                   r.Zone,
		State:                  upcloud.ManagedDatabaseStateRunning,
		Powered:                true,
		NodeCount:              1,
		Networks:               r.Networks,
		TerminationProtection:  r.TerminationProtection != nil && *r.TerminationProtection,
		AdditionalDiskSpaceGiB: r.AdditionalDiskSpaceGiB,
		Labels:                 r.Labels,
		Properties:             upcloud.ManagedDatabaseProperties(r.Properties),
		ServiceURIParams:       params,
		ServiceURI:             f.composedURI(params),
		Users:                  []upcloud.ManagedDatabaseUser{{Username: "upadmin", Type: upcloud.ManagedDatabaseUserTypePrimary}},
	}
	f.Databases[uuid] = db
	cp := *db
	return &cp, nil
}

// GetManagedDatabase implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) GetManagedDatabase(_ context.Context, r *request.GetManagedDatabaseRequest) (*upcloud.ManagedDatabase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedDatabase")
	db, ok := f.Databases[r.UUID]
	if !ok {
		return nil, NotFound("database")
	}
	return f.copy(db), nil
}

// GetAllManagedDatabases implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) GetAllManagedDatabases(_ context.Context) ([]upcloud.ManagedDatabase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetAllManagedDatabases")
	out := make([]upcloud.ManagedDatabase, 0, len(f.Databases))
	for _, db := range f.Databases {
		cp := *f.copy(db)
		out = append(out, cp)
	}
	return out, nil
}

// ModifyManagedDatabase implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) ModifyManagedDatabase(_ context.Context, r *request.ModifyManagedDatabaseRequest) (*upcloud.ManagedDatabase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyManagedDatabase"); err != nil {
		return nil, err
	}
	db, ok := f.Databases[r.UUID]
	if !ok {
		return nil, NotFound("database")
	}
	if r.Plan != "" {
		db.Plan = r.Plan
	}
	if r.Title != "" {
		db.Title = r.Title
	}
	if r.AdditionalDiskSpaceGiB != nil {
		db.AdditionalDiskSpaceGiB = *r.AdditionalDiskSpaceGiB
	}
	if r.TerminationProtection != nil {
		db.TerminationProtection = *r.TerminationProtection
	}
	if r.Labels != nil {
		db.Labels = *r.Labels
	}
	if r.Networks != nil {
		db.Networks = *r.Networks
	}
	if r.Maintenance.DayOfWeek != "" || r.Maintenance.Time != "" {
		db.Maintenance = upcloud.ManagedDatabaseMaintenanceTime{DayOfWeek: r.Maintenance.DayOfWeek, Time: r.Maintenance.Time}
	}
	if len(r.Properties) > 0 {
		if db.Properties == nil {
			db.Properties = upcloud.ManagedDatabaseProperties{}
		}
		for k, v := range r.Properties {
			db.Properties[k] = v
		}
	}
	cp := *db
	return &cp, nil
}

// DeleteManagedDatabase implements upcloudapi.DatabaseAPI.
//
// The first call moves the service to the delete-service state and reports
// success, mirroring the real API where the resource stays visible for a
// while. A subsequent call finds the service already deleting, removes it
// and reports 404, so the adapter observes it as gone.
func (f *DatabaseAPI) DeleteManagedDatabase(_ context.Context, r *request.DeleteManagedDatabaseRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteManagedDatabase"); err != nil {
		return err
	}
	db, ok := f.Databases[r.UUID]
	if !ok {
		return NotFound("database")
	}
	if db.TerminationProtection {
		return &upcloud.Problem{Status: http.StatusConflict, Title: "database deletion refused, termination protection is enabled"}
	}
	if db.State == upcloud.ManagedDatabaseStateDeleteService {
		delete(f.Databases, r.UUID)
		return NotFound("database")
	}
	db.State = upcloud.ManagedDatabaseStateDeleteService
	return nil
}

// StartManagedDatabase implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) StartManagedDatabase(_ context.Context, r *request.StartManagedDatabaseRequest) (*upcloud.ManagedDatabase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("StartManagedDatabase"); err != nil {
		return nil, err
	}
	db, ok := f.Databases[r.UUID]
	if !ok {
		return nil, NotFound("database")
	}
	db.Powered = true
	db.State = upcloud.ManagedDatabaseStateRunning
	cp := *db
	return &cp, nil
}

// ShutdownManagedDatabase implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) ShutdownManagedDatabase(_ context.Context, r *request.ShutdownManagedDatabaseRequest) (*upcloud.ManagedDatabase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ShutdownManagedDatabase"); err != nil {
		return nil, err
	}
	db, ok := f.Databases[r.UUID]
	if !ok {
		return nil, NotFound("database")
	}
	db.Powered = false
	db.State = upcloud.ManagedDatabaseStateStopped
	cp := *db
	return &cp, nil
}

func (f *DatabaseAPI) findUser(db *upcloud.ManagedDatabase, username string) *upcloud.ManagedDatabaseUser {
	for i := range db.Users {
		if db.Users[i].Username == username {
			return &db.Users[i]
		}
	}
	return nil
}

// CreateManagedDatabaseUser implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) CreateManagedDatabaseUser(_ context.Context, r *request.CreateManagedDatabaseUserRequest) (*upcloud.ManagedDatabaseUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateManagedDatabaseUser"); err != nil {
		return nil, err
	}
	db, ok := f.Databases[r.ServiceUUID]
	if !ok {
		return nil, NotFound("database")
	}
	if f.findUser(db, r.Username) != nil {
		return nil, Conflict("database user")
	}
	password := r.Password
	if password == "" {
		password = "gen-" + r.Username
	}
	user := upcloud.ManagedDatabaseUser{
		Username:                r.Username,
		Type:                    upcloud.ManagedDatabaseUserTypeNormal,
		Password:                password,
		Authentication:          r.Authentication,
		PGAccessControl:         r.PGAccessControl,
		ValkeyAccessControl:     r.ValkeyAccessControl,
		OpenSearchAccessControl: r.OpenSearchAccessControl,
	}
	db.Users = append(db.Users, user)
	cp := user
	return &cp, nil
}

// GetManagedDatabaseUser implements upcloudapi.DatabaseAPI.
// Unlike the list endpoints, the single-user GET returns the password.
func (f *DatabaseAPI) GetManagedDatabaseUser(_ context.Context, r *request.GetManagedDatabaseUserRequest) (*upcloud.ManagedDatabaseUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedDatabaseUser")
	db, ok := f.Databases[r.ServiceUUID]
	if !ok {
		return nil, NotFound("database")
	}
	user := f.findUser(db, r.Username)
	if user == nil {
		return nil, NotFound("database user")
	}
	cp := *user
	return &cp, nil
}

// ModifyManagedDatabaseUser implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) ModifyManagedDatabaseUser(_ context.Context, r *request.ModifyManagedDatabaseUserRequest) (*upcloud.ManagedDatabaseUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyManagedDatabaseUser"); err != nil {
		return nil, err
	}
	db, ok := f.Databases[r.ServiceUUID]
	if !ok {
		return nil, NotFound("database")
	}
	user := f.findUser(db, r.Username)
	if user == nil {
		return nil, NotFound("database user")
	}
	if r.Password != "" {
		user.Password = r.Password
	}
	user.Authentication = r.Authentication
	cp := *user
	return &cp, nil
}

// ModifyManagedDatabaseUserAccessControl implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) ModifyManagedDatabaseUserAccessControl(_ context.Context, r *request.ModifyManagedDatabaseUserAccessControlRequest) (*upcloud.ManagedDatabaseUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyManagedDatabaseUserAccessControl"); err != nil {
		return nil, err
	}
	db, ok := f.Databases[r.ServiceUUID]
	if !ok {
		return nil, NotFound("database")
	}
	user := f.findUser(db, r.Username)
	if user == nil {
		return nil, NotFound("database user")
	}
	user.PGAccessControl = r.PGAccessControl
	user.ValkeyAccessControl = r.ValkeyAccessControl
	user.OpenSearchAccessControl = r.OpenSearchAccessControl
	cp := *user
	return &cp, nil
}

// DeleteManagedDatabaseUser implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) DeleteManagedDatabaseUser(_ context.Context, r *request.DeleteManagedDatabaseUserRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteManagedDatabaseUser"); err != nil {
		return err
	}
	db, ok := f.Databases[r.ServiceUUID]
	if !ok {
		return NotFound("database")
	}
	for i := range db.Users {
		if db.Users[i].Username == r.Username {
			if db.Users[i].Type == upcloud.ManagedDatabaseUserTypePrimary {
				return &upcloud.Problem{Status: http.StatusConflict, Title: "primary user cannot be deleted"}
			}
			db.Users = removeAt(db.Users, i)
			return nil
		}
	}
	return NotFound("database user")
}

// CreateManagedDatabaseLogicalDatabase implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) CreateManagedDatabaseLogicalDatabase(_ context.Context, r *request.CreateManagedDatabaseLogicalDatabaseRequest) (*upcloud.ManagedDatabaseLogicalDatabase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateManagedDatabaseLogicalDatabase"); err != nil {
		return nil, err
	}
	if _, ok := f.Databases[r.ServiceUUID]; !ok {
		return nil, NotFound("database")
	}
	for _, d := range f.LogicalDBs[r.ServiceUUID] {
		if d.Name == r.Name {
			return nil, Conflict("logical database")
		}
	}
	db := upcloud.ManagedDatabaseLogicalDatabase{Name: r.Name, LCCollate: r.LCCollate, LCCType: r.LCCType}
	f.LogicalDBs[r.ServiceUUID] = append(f.LogicalDBs[r.ServiceUUID], db)
	cp := db
	return &cp, nil
}

// GetManagedDatabaseLogicalDatabases implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) GetManagedDatabaseLogicalDatabases(_ context.Context, r *request.GetManagedDatabaseLogicalDatabasesRequest) ([]upcloud.ManagedDatabaseLogicalDatabase, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedDatabaseLogicalDatabases")
	if _, ok := f.Databases[r.ServiceUUID]; !ok {
		return nil, NotFound("database")
	}
	out := make([]upcloud.ManagedDatabaseLogicalDatabase, len(f.LogicalDBs[r.ServiceUUID]))
	copy(out, f.LogicalDBs[r.ServiceUUID])
	return out, nil
}

// DeleteManagedDatabaseLogicalDatabase implements upcloudapi.DatabaseAPI.
func (f *DatabaseAPI) DeleteManagedDatabaseLogicalDatabase(_ context.Context, r *request.DeleteManagedDatabaseLogicalDatabaseRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteManagedDatabaseLogicalDatabase"); err != nil {
		return err
	}
	if _, ok := f.Databases[r.ServiceUUID]; !ok {
		return NotFound("database")
	}
	list := f.LogicalDBs[r.ServiceUUID]
	for i := range list {
		if list[i].Name == r.Name {
			f.LogicalDBs[r.ServiceUUID] = removeAt(list, i)
			return nil
		}
	}
	return NotFound("logical database")
}

func removeAt[T any](in []T, i int) []T {
	out := make([]T, 0, len(in)-1)
	return append(append(out, in[:i]...), in[i+1:]...)
}
