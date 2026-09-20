package fake

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

var _ upcloudapi.ObjectStorageAPI = (*ObjectStorageAPI)(nil)

// ObjectStorageAPI is an in-memory upcloudapi.ObjectStorageAPI.
//
// Services are stored in Services keyed by UUID. Children are stored in side
// maps keyed by service UUID (matching the real API, which addresses them by
// service UUID plus name or key id). Access key secrets are kept in
// KeySecrets: CreateManagedObjectStorageUserAccessKey returns the secret once
// (as a pointer), subsequent Gets report it nil, mirroring the real API.
type ObjectStorageAPI struct {
	mu       sync.Mutex
	seq      int
	Services map[string]*upcloud.ManagedObjectStorage
	Users    map[string][]upcloud.ManagedObjectStorageUser
	Policies map[string][]upcloud.ManagedObjectStoragePolicy
	Buckets  map[string][]upcloud.ManagedObjectStorageBucketMetrics
	// KeySecrets maps an access key ID to its secret. The secret is only
	// returned by the create call, never by the Get endpoints.
	KeySecrets map[string]string
	Calls      []string // method names in call order
	// StateOverride, when set, is the OperationalState reported by the GET
	// endpoints in place of the stored state (to simulate transitional states).
	StateOverride upcloud.ManagedObjectStorageOperationalState
	// FailNext, when set, is returned by the next mutating call and cleared.
	FailNext error
}

// NewObjectStorageAPI returns an empty fake.
func NewObjectStorageAPI() *ObjectStorageAPI {
	return &ObjectStorageAPI{
		Services:   map[string]*upcloud.ManagedObjectStorage{},
		Users:      map[string][]upcloud.ManagedObjectStorageUser{},
		Policies:   map[string][]upcloud.ManagedObjectStoragePolicy{},
		Buckets:    map[string][]upcloud.ManagedObjectStorageBucketMetrics{},
		KeySecrets: map[string]string{},
	}
}

// record appends the call to Calls. Callers must hold f.mu.
func (f *ObjectStorageAPI) record(name string) error {
	f.Calls = append(f.Calls, name)
	if f.FailNext != nil {
		err := f.FailNext
		f.FailNext = nil
		return err
	}
	return nil
}

func (f *ObjectStorageAPI) nextUUID(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%04d", prefix, f.seq)
}

// copyService applies StateOverride to a copy of the service so tests can
// simulate a transitional state without mutating the stored resource.
func (f *ObjectStorageAPI) copyService(svc *upcloud.ManagedObjectStorage) *upcloud.ManagedObjectStorage {
	cp := *svc
	if f.StateOverride != "" {
		cp.OperationalState = f.StateOverride
	}
	return &cp
}

func (f *ObjectStorageAPI) findUser(svcUUID, username string) *upcloud.ManagedObjectStorageUser {
	for i := range f.Users[svcUUID] {
		if f.Users[svcUUID][i].Username == username {
			return &f.Users[svcUUID][i]
		}
	}
	return nil
}

func (f *ObjectStorageAPI) findKey(svcUUID, username, accessKeyID string) *upcloud.ManagedObjectStorageUserAccessKey {
	u := f.findUser(svcUUID, username)
	if u == nil {
		return nil
	}
	for i := range u.AccessKeys {
		if u.AccessKeys[i].AccessKeyID == accessKeyID {
			return &u.AccessKeys[i]
		}
	}
	return nil
}

func (f *ObjectStorageAPI) findPolicy(svcUUID, name string) *upcloud.ManagedObjectStoragePolicy {
	for i := range f.Policies[svcUUID] {
		if f.Policies[svcUUID][i].Name == name {
			return &f.Policies[svcUUID][i]
		}
	}
	return nil
}

func (f *ObjectStorageAPI) findBucket(svcUUID, name string) *upcloud.ManagedObjectStorageBucketMetrics {
	for i := range f.Buckets[svcUUID] {
		if f.Buckets[svcUUID][i].Name == name {
			return &f.Buckets[svcUUID][i]
		}
	}
	return nil
}

func (f *ObjectStorageAPI) findDomain(svcUUID, domainName string) *upcloud.ManagedObjectStorageCustomDomain {
	svc, ok := f.Services[svcUUID]
	if !ok {
		return nil
	}
	for i := range svc.CustomDomains {
		if svc.CustomDomains[i].DomainName == domainName {
			return &svc.CustomDomains[i]
		}
	}
	return nil
}

// CreateManagedObjectStorage implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) CreateManagedObjectStorage(_ context.Context, r *request.CreateManagedObjectStorageRequest) (*upcloud.ManagedObjectStorage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateManagedObjectStorage"); err != nil {
		return nil, err
	}
	uuid := f.nextUUID("moss")
	operational := upcloud.ManagedObjectStorageOperationalStateRunning
	if r.ConfiguredStatus == upcloud.ManagedObjectStorageConfiguredStatusStopped {
		operational = upcloud.ManagedObjectStorageOperationalStateStopped
	}
	svc := &upcloud.ManagedObjectStorage{
		UUID:                  uuid,
		Name:                  r.Name,
		Region:                r.Region,
		ConfiguredStatus:      r.ConfiguredStatus,
		OperationalState:      operational,
		TerminationProtection: r.TerminationProtection,
		Labels:                r.Labels,
		Networks:              r.Networks,
		Endpoints:             []upcloud.ManagedObjectStorageEndpoint{{DomainName: uuid + ".upcloudobjects.com", Type: "public"}},
	}
	f.Services[uuid] = svc
	cp := *svc
	return &cp, nil
}

// GetManagedObjectStorage implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) GetManagedObjectStorage(_ context.Context, r *request.GetManagedObjectStorageRequest) (*upcloud.ManagedObjectStorage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedObjectStorage")
	svc, ok := f.Services[r.UUID]
	if !ok {
		return nil, NotFound("object storage")
	}
	return f.copyService(svc), nil
}

// GetManagedObjectStorages implements upcloudapi.ObjectStorageAPI. The real
// endpoint takes no label filter, so the fake returns every service.
func (f *ObjectStorageAPI) GetManagedObjectStorages(_ context.Context, _ *request.GetManagedObjectStoragesRequest) ([]upcloud.ManagedObjectStorage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedObjectStorages")
	out := make([]upcloud.ManagedObjectStorage, 0, len(f.Services))
	for _, svc := range f.Services {
		cp := *f.copyService(svc)
		out = append(out, cp)
	}
	return out, nil
}

// ModifyManagedObjectStorage implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) ModifyManagedObjectStorage(_ context.Context, r *request.ModifyManagedObjectStorageRequest) (*upcloud.ManagedObjectStorage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyManagedObjectStorage"); err != nil {
		return nil, err
	}
	svc, ok := f.Services[r.UUID]
	if !ok {
		return nil, NotFound("object storage")
	}
	if r.Name != nil {
		svc.Name = *r.Name
	}
	if r.ConfiguredStatus != nil {
		svc.ConfiguredStatus = *r.ConfiguredStatus
		switch *r.ConfiguredStatus {
		case upcloud.ManagedObjectStorageConfiguredStatusStarted:
			svc.OperationalState = upcloud.ManagedObjectStorageOperationalStateRunning
		case upcloud.ManagedObjectStorageConfiguredStatusStopped:
			svc.OperationalState = upcloud.ManagedObjectStorageOperationalStateStopped
		}
	}
	if r.TerminationProtection != nil {
		svc.TerminationProtection = *r.TerminationProtection
	}
	if r.Labels != nil {
		svc.Labels = *r.Labels
	}
	if r.Networks != nil {
		svc.Networks = *r.Networks
	}
	cp := *svc
	return &cp, nil
}

// DeleteManagedObjectStorage implements upcloudapi.ObjectStorageAPI. A
// service with buckets cannot be deleted (409). A successful delete removes
// the service immediately, mirroring the visible state the adapter polls for.
func (f *ObjectStorageAPI) DeleteManagedObjectStorage(_ context.Context, r *request.DeleteManagedObjectStorageRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteManagedObjectStorage"); err != nil {
		return err
	}
	svc, ok := f.Services[r.UUID]
	if !ok {
		return NotFound("object storage")
	}
	if svc.TerminationProtection {
		return &upcloud.Problem{Status: http.StatusConflict, Title: "deletion refused, termination protection is enabled"}
	}
	if len(f.Buckets[r.UUID]) > 0 {
		return &upcloud.Problem{Status: http.StatusConflict, Title: "service has buckets; delete them first"}
	}
	// Removing the service also removes its children.
	delete(f.Services, r.UUID)
	delete(f.Users, r.UUID)
	delete(f.Policies, r.UUID)
	delete(f.Buckets, r.UUID)
	return nil
}

// CreateManagedObjectStorageUser implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) CreateManagedObjectStorageUser(_ context.Context, r *request.CreateManagedObjectStorageUserRequest) (*upcloud.ManagedObjectStorageUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateManagedObjectStorageUser"); err != nil {
		return nil, err
	}
	svc, ok := f.Services[r.ServiceUUID]
	if !ok {
		return nil, NotFound("object storage")
	}
	if f.findUser(r.ServiceUUID, r.Username) != nil {
		return nil, Conflict("object storage user")
	}
	user := upcloud.ManagedObjectStorageUser{
		Username: r.Username,
		ARN:      fmt.Sprintf("arn:upcloud:objectstorage:%s::user/%s", svc.Region, r.Username),
	}
	f.Users[r.ServiceUUID] = append(f.Users[r.ServiceUUID], user)
	stored := f.findUser(r.ServiceUUID, r.Username)
	cp := *stored
	return &cp, nil
}

// GetManagedObjectStorageUser implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) GetManagedObjectStorageUser(_ context.Context, r *request.GetManagedObjectStorageUserRequest) (*upcloud.ManagedObjectStorageUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedObjectStorageUser")
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return nil, NotFound("object storage")
	}
	user := f.findUser(r.ServiceUUID, r.Username)
	if user == nil {
		return nil, NotFound("object storage user")
	}
	cp := *user
	return &cp, nil
}

// GetManagedObjectStorageUsers implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) GetManagedObjectStorageUsers(_ context.Context, r *request.GetManagedObjectStorageUsersRequest) ([]upcloud.ManagedObjectStorageUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedObjectStorageUsers")
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return nil, NotFound("object storage")
	}
	list := f.Users[r.ServiceUUID]
	out := make([]upcloud.ManagedObjectStorageUser, len(list))
	copy(out, list)
	return out, nil
}

// DeleteManagedObjectStorageUser implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) DeleteManagedObjectStorageUser(_ context.Context, r *request.DeleteManagedObjectStorageUserRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteManagedObjectStorageUser"); err != nil {
		return err
	}
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return NotFound("object storage")
	}
	list := f.Users[r.ServiceUUID]
	for i := range list {
		if list[i].Username == r.Username {
			// Drop the user's access key secrets.
			for _, k := range list[i].AccessKeys {
				delete(f.KeySecrets, k.AccessKeyID)
			}
			f.Users[r.ServiceUUID] = removeAt(list, i)
			return nil
		}
	}
	return NotFound("object storage user")
}

// AttachManagedObjectStorageUserPolicy implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) AttachManagedObjectStorageUserPolicy(_ context.Context, r *request.AttachManagedObjectStorageUserPolicyRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("AttachManagedObjectStorageUserPolicy"); err != nil {
		return err
	}
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return NotFound("object storage")
	}
	user := f.findUser(r.ServiceUUID, r.Username)
	if user == nil {
		return NotFound("object storage user")
	}
	pol := f.findPolicy(r.ServiceUUID, r.Name)
	if pol == nil {
		return NotFound("object storage policy")
	}
	for _, p := range user.Policies {
		if p.Name == r.Name {
			return nil // already attached
		}
	}
	user.Policies = append(user.Policies, *pol)
	return nil
}

// DetachManagedObjectStorageUserPolicy implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) DetachManagedObjectStorageUserPolicy(_ context.Context, r *request.DetachManagedObjectStorageUserPolicyRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DetachManagedObjectStorageUserPolicy"); err != nil {
		return err
	}
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return NotFound("object storage")
	}
	user := f.findUser(r.ServiceUUID, r.Username)
	if user == nil {
		return NotFound("object storage user")
	}
	out := user.Policies[:0]
	removed := false
	for _, p := range user.Policies {
		if p.Name == r.Name {
			removed = true
			continue
		}
		out = append(out, p)
	}
	if !removed {
		return NotFound("object storage user policy")
	}
	user.Policies = out
	return nil
}

// GetManagedObjectStorageUserPolicies implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) GetManagedObjectStorageUserPolicies(_ context.Context, r *request.GetManagedObjectStorageUserPoliciesRequest) ([]upcloud.ManagedObjectStorageUserPolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedObjectStorageUserPolicies")
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return nil, NotFound("object storage")
	}
	user := f.findUser(r.ServiceUUID, r.Username)
	if user == nil {
		return nil, NotFound("object storage user")
	}
	out := make([]upcloud.ManagedObjectStorageUserPolicy, len(user.Policies))
	for i, p := range user.Policies {
		out[i] = upcloud.ManagedObjectStorageUserPolicy{ARN: p.ARN, Name: p.Name}
	}
	return out, nil
}

// CreateManagedObjectStorageUserAccessKey implements upcloudapi.ObjectStorageAPI.
// The secret is returned once here and never by the Get endpoints.
func (f *ObjectStorageAPI) CreateManagedObjectStorageUserAccessKey(_ context.Context, r *request.CreateManagedObjectStorageUserAccessKeyRequest) (*upcloud.ManagedObjectStorageUserAccessKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateManagedObjectStorageUserAccessKey"); err != nil {
		return nil, err
	}
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return nil, NotFound("object storage")
	}
	user := f.findUser(r.ServiceUUID, r.Username)
	if user == nil {
		return nil, NotFound("object storage user")
	}
	f.seq++
	keyID := fmt.Sprintf("AKIA%016d", f.seq)
	secret := "fake-s3-secret-" + keyID
	key := upcloud.ManagedObjectStorageUserAccessKey{
		AccessKeyID: keyID,
		Status:      upcloud.ManagedObjectStorageUserAccessKeyStatusActive,
	}
	user.AccessKeys = append(user.AccessKeys, key)
	f.KeySecrets[keyID] = secret
	stored := f.findKey(r.ServiceUUID, r.Username, keyID)
	cp := *stored
	secretCopy := secret
	cp.SecretAccessKey = &secretCopy
	return &cp, nil
}

// GetManagedObjectStorageUserAccessKey implements upcloudapi.ObjectStorageAPI.
// The secret is not returned (it is nil), mirroring the real API.
func (f *ObjectStorageAPI) GetManagedObjectStorageUserAccessKey(_ context.Context, r *request.GetManagedObjectStorageUserAccessKeyRequest) (*upcloud.ManagedObjectStorageUserAccessKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedObjectStorageUserAccessKey")
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return nil, NotFound("object storage")
	}
	key := f.findKey(r.ServiceUUID, r.Username, r.AccessKeyID)
	if key == nil {
		return nil, NotFound("object storage access key")
	}
	cp := *key
	return &cp, nil
}

// GetManagedObjectStorageUserAccessKeys implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) GetManagedObjectStorageUserAccessKeys(_ context.Context, r *request.GetManagedObjectStorageUserAccessKeysRequest) ([]upcloud.ManagedObjectStorageUserAccessKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedObjectStorageUserAccessKeys")
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return nil, NotFound("object storage")
	}
	user := f.findUser(r.ServiceUUID, r.Username)
	if user == nil {
		return nil, NotFound("object storage user")
	}
	out := make([]upcloud.ManagedObjectStorageUserAccessKey, len(user.AccessKeys))
	copy(out, user.AccessKeys)
	return out, nil
}

// ModifyManagedObjectStorageUserAccessKey implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) ModifyManagedObjectStorageUserAccessKey(_ context.Context, r *request.ModifyManagedObjectStorageUserAccessKeyRequest) (*upcloud.ManagedObjectStorageUserAccessKey, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyManagedObjectStorageUserAccessKey"); err != nil {
		return nil, err
	}
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return nil, NotFound("object storage")
	}
	key := f.findKey(r.ServiceUUID, r.Username, r.AccessKeyID)
	if key == nil {
		return nil, NotFound("object storage access key")
	}
	if r.Status != "" {
		key.Status = r.Status
	}
	cp := *key
	return &cp, nil
}

// DeleteManagedObjectStorageUserAccessKey implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) DeleteManagedObjectStorageUserAccessKey(_ context.Context, r *request.DeleteManagedObjectStorageUserAccessKeyRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteManagedObjectStorageUserAccessKey"); err != nil {
		return err
	}
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return NotFound("object storage")
	}
	user := f.findUser(r.ServiceUUID, r.Username)
	if user == nil {
		return NotFound("object storage user")
	}
	for i := range user.AccessKeys {
		if user.AccessKeys[i].AccessKeyID == r.AccessKeyID {
			delete(f.KeySecrets, r.AccessKeyID)
			user.AccessKeys = removeAt(user.AccessKeys, i)
			return nil
		}
	}
	return NotFound("object storage access key")
}

// invalidPolicyResource returns the first statement Resource in doc that uses
// the invented arn:upcloud: namespace, which the real API rejects with 400. A
// document that does not parse is not checked further.
func invalidPolicyResource(doc string) (string, bool) {
	var d struct {
		Statement json.RawMessage `json:"Statement"`
	}
	if json.Unmarshal([]byte(doc), &d) != nil {
		return "", false
	}
	var stmts []struct {
		Resource json.RawMessage `json:"Resource"`
	}
	if json.Unmarshal(d.Statement, &stmts) != nil {
		var one struct {
			Resource json.RawMessage `json:"Resource"`
		}
		if json.Unmarshal(d.Statement, &one) != nil {
			return "", false
		}
		stmts = append(stmts, one)
	}
	for _, st := range stmts {
		var many []string
		if json.Unmarshal(st.Resource, &many) != nil {
			var single string
			if json.Unmarshal(st.Resource, &single) != nil {
				continue
			}
			many = []string{single}
		}
		for _, r := range many {
			if strings.HasPrefix(r, "arn:upcloud:") {
				return r, true
			}
		}
	}
	return "", false
}

// CreateManagedObjectStoragePolicy implements upcloudapi.ObjectStorageAPI.
// The document is stored and returned as the raw string, unchanged. Resource
// values in the arn:upcloud: namespace are rejected with 400, as the real API
// does; other values are not validated.
func (f *ObjectStorageAPI) CreateManagedObjectStoragePolicy(_ context.Context, r *request.CreateManagedObjectStoragePolicyRequest) (*upcloud.ManagedObjectStoragePolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateManagedObjectStoragePolicy"); err != nil {
		return nil, err
	}
	svc, ok := f.Services[r.ServiceUUID]
	if !ok {
		return nil, NotFound("object storage")
	}
	if res, bad := invalidPolicyResource(r.Document); bad {
		return nil, Invalid(fmt.Sprintf("Policy has invalid resource: '%s'", res))
	}
	if f.findPolicy(r.ServiceUUID, r.Name) != nil {
		return nil, Conflict("object storage policy")
	}
	f.seq++
	pol := upcloud.ManagedObjectStoragePolicy{
		Name:             r.Name,
		Description:      r.Description,
		Document:         r.Document,
		ARN:              fmt.Sprintf("arn:upcloud:objectstorage:%s::policy/%s", svc.Region, r.Name),
		DefaultVersionID: fmt.Sprintf("v%04d", f.seq),
		System:           false,
	}
	f.Policies[r.ServiceUUID] = append(f.Policies[r.ServiceUUID], pol)
	stored := f.findPolicy(r.ServiceUUID, r.Name)
	cp := *stored
	return &cp, nil
}

// GetManagedObjectStoragePolicy implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) GetManagedObjectStoragePolicy(_ context.Context, r *request.GetManagedObjectStoragePolicyRequest) (*upcloud.ManagedObjectStoragePolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedObjectStoragePolicy")
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return nil, NotFound("object storage")
	}
	pol := f.findPolicy(r.ServiceUUID, r.Name)
	if pol == nil {
		return nil, NotFound("object storage policy")
	}
	cp := *pol
	return &cp, nil
}

// GetManagedObjectStoragePolicies implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) GetManagedObjectStoragePolicies(_ context.Context, r *request.GetManagedObjectStoragePoliciesRequest) ([]upcloud.ManagedObjectStoragePolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedObjectStoragePolicies")
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return nil, NotFound("object storage")
	}
	list := f.Policies[r.ServiceUUID]
	out := make([]upcloud.ManagedObjectStoragePolicy, len(list))
	copy(out, list)
	return out, nil
}

// DeleteManagedObjectStoragePolicy implements upcloudapi.ObjectStorageAPI.
// A policy that is attached to any user cannot be deleted (409).
func (f *ObjectStorageAPI) DeleteManagedObjectStoragePolicy(_ context.Context, r *request.DeleteManagedObjectStoragePolicyRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteManagedObjectStoragePolicy"); err != nil {
		return err
	}
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return NotFound("object storage")
	}
	// A policy in use cannot be deleted.
	for _, u := range f.Users[r.ServiceUUID] {
		for _, p := range u.Policies {
			if p.Name == r.Name {
				return &upcloud.Problem{Status: http.StatusConflict, Title: "policy is attached to a user"}
			}
		}
	}
	list := f.Policies[r.ServiceUUID]
	for i := range list {
		if list[i].Name == r.Name {
			f.Policies[r.ServiceUUID] = removeAt(list, i)
			return nil
		}
	}
	return NotFound("object storage policy")
}

// CreateManagedObjectStorageBucket implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) CreateManagedObjectStorageBucket(_ context.Context, r *request.CreateManagedObjectStorageBucketRequest) (upcloud.ManagedObjectStorageBucketMetrics, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateManagedObjectStorageBucket"); err != nil {
		return upcloud.ManagedObjectStorageBucketMetrics{}, err
	}
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return upcloud.ManagedObjectStorageBucketMetrics{}, NotFound("object storage")
	}
	if f.findBucket(r.ServiceUUID, r.Name) != nil {
		return upcloud.ManagedObjectStorageBucketMetrics{}, Conflict("object storage bucket")
	}
	b := upcloud.ManagedObjectStorageBucketMetrics{Name: r.Name, Deleted: false}
	f.Buckets[r.ServiceUUID] = append(f.Buckets[r.ServiceUUID], b)
	return b, nil
}

// GetManagedObjectStorageBucketMetrics implements upcloudapi.ObjectStorageAPI.
// It honours r.Page for pagination: Page.Number is the 1-based page index and
// Page.Size the page length; an out-of-range page yields an empty slice.
func (f *ObjectStorageAPI) GetManagedObjectStorageBucketMetrics(_ context.Context, r *request.GetManagedObjectStorageBucketMetricsRequest) ([]upcloud.ManagedObjectStorageBucketMetrics, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedObjectStorageBucketMetrics")
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return nil, NotFound("object storage")
	}
	all := f.Buckets[r.ServiceUUID]
	if r.Page == nil || r.Page.Size == 0 {
		out := make([]upcloud.ManagedObjectStorageBucketMetrics, len(all))
		copy(out, all)
		return out, nil
	}
	size := r.Page.Size
	start := (r.Page.Number - 1) * size
	if start >= len(all) {
		return []upcloud.ManagedObjectStorageBucketMetrics{}, nil
	}
	end := min(start+size, len(all))
	out := make([]upcloud.ManagedObjectStorageBucketMetrics, end-start)
	copy(out, all[start:end])
	return out, nil
}

// DeleteManagedObjectStorageBucket implements upcloudapi.ObjectStorageAPI.
// A non-empty bucket cannot be deleted (409). A successful delete removes the
// bucket from the metrics, mirroring the API deleting it asynchronously; a
// subsequent delete of an already-gone bucket reports 404.
func (f *ObjectStorageAPI) DeleteManagedObjectStorageBucket(_ context.Context, r *request.DeleteManagedObjectStorageBucketRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteManagedObjectStorageBucket"); err != nil {
		return err
	}
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return NotFound("object storage")
	}
	list := f.Buckets[r.ServiceUUID]
	for i := range list {
		if list[i].Name == r.Name {
			if list[i].TotalObjects > 0 {
				return &upcloud.Problem{Status: http.StatusConflict, Title: "bucket is not empty"}
			}
			f.Buckets[r.ServiceUUID] = removeAt(list, i)
			return nil
		}
	}
	return NotFound("object storage bucket")
}

// CreateManagedObjectStorageCustomDomain implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) CreateManagedObjectStorageCustomDomain(_ context.Context, r *request.CreateManagedObjectStorageCustomDomainRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateManagedObjectStorageCustomDomain"); err != nil {
		return err
	}
	svc, ok := f.Services[r.ServiceUUID]
	if !ok {
		return NotFound("object storage")
	}
	if f.findDomain(r.ServiceUUID, r.DomainName) != nil {
		return Conflict("object storage custom domain")
	}
	svc.CustomDomains = append(svc.CustomDomains, upcloud.ManagedObjectStorageCustomDomain{DomainName: r.DomainName, Type: r.Type})
	return nil
}

// GetManagedObjectStorageCustomDomain implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) GetManagedObjectStorageCustomDomain(_ context.Context, r *request.GetManagedObjectStorageCustomDomainRequest) (*upcloud.ManagedObjectStorageCustomDomain, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedObjectStorageCustomDomain")
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return nil, NotFound("object storage")
	}
	d := f.findDomain(r.ServiceUUID, r.DomainName)
	if d == nil {
		return nil, NotFound("object storage custom domain")
	}
	cp := *d
	return &cp, nil
}

// GetManagedObjectStorageCustomDomains implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) GetManagedObjectStorageCustomDomains(_ context.Context, r *request.GetManagedObjectStorageCustomDomainsRequest) ([]upcloud.ManagedObjectStorageCustomDomain, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, "GetManagedObjectStorageCustomDomains")
	svc, ok := f.Services[r.ServiceUUID]
	if !ok {
		return nil, NotFound("object storage")
	}
	out := make([]upcloud.ManagedObjectStorageCustomDomain, len(svc.CustomDomains))
	copy(out, svc.CustomDomains)
	return out, nil
}

// ModifyManagedObjectStorageCustomDomain implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) ModifyManagedObjectStorageCustomDomain(_ context.Context, r *request.ModifyManagedObjectStorageCustomDomainRequest) (*upcloud.ManagedObjectStorageCustomDomain, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyManagedObjectStorageCustomDomain"); err != nil {
		return nil, err
	}
	if _, ok := f.Services[r.ServiceUUID]; !ok {
		return nil, NotFound("object storage")
	}
	d := f.findDomain(r.ServiceUUID, r.DomainName)
	if d == nil {
		return nil, NotFound("object storage custom domain")
	}
	d.Type = r.CustomDomain.Type
	cp := *d
	return &cp, nil
}

// DeleteManagedObjectStorageCustomDomain implements upcloudapi.ObjectStorageAPI.
func (f *ObjectStorageAPI) DeleteManagedObjectStorageCustomDomain(_ context.Context, r *request.DeleteManagedObjectStorageCustomDomainRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteManagedObjectStorageCustomDomain"); err != nil {
		return err
	}
	svc, ok := f.Services[r.ServiceUUID]
	if !ok {
		return NotFound("object storage")
	}
	for i := range svc.CustomDomains {
		if svc.CustomDomains[i].DomainName == r.DomainName {
			svc.CustomDomains = removeAt(svc.CustomDomains, i)
			return nil
		}
	}
	return NotFound("object storage custom domain")
}
