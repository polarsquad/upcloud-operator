package fake

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"

	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

var _ upcloudapi.LoadBalancerAPI = (*LoadBalancerAPI)(nil)

// LoadBalancerAPI is an in-memory upcloudapi.LoadBalancerAPI.
//
// Load balancers are stored in LoadBalancers keyed by UUID with their children
// inline exactly as the API returns them (Frontends, Backends, Resolvers, and
// nested Members / TLSConfigs / Rules). Certificate bundles are account-level
// and stored separately in CertificateBundles keyed by UUID.
type LoadBalancerAPI struct {
	mu                 sync.Mutex
	seq                int
	LoadBalancers      map[string]*upcloud.LoadBalancer
	CertificateBundles map[string]*upcloud.LoadBalancerCertificateBundle
	Calls              []string
	// StateOverride, when set, is the OperationalState reported by the service
	// GET endpoints in place of the stored state.
	StateOverride upcloud.LoadBalancerOperationalState
	// BundleStateOverride mirrors StateOverride for certificate bundles.
	BundleStateOverride upcloud.LoadBalancerCertificateBundleOperationalState
	// FailNext, when set, is returned by the next mutating call and cleared.
	FailNext error
}

// NewLoadBalancerAPI returns an empty fake.
func NewLoadBalancerAPI() *LoadBalancerAPI {
	return &LoadBalancerAPI{
		LoadBalancers:      map[string]*upcloud.LoadBalancer{},
		CertificateBundles: map[string]*upcloud.LoadBalancerCertificateBundle{},
	}
}

// record appends the call to Calls. Callers must hold f.mu.
func (f *LoadBalancerAPI) record(name string) error {
	f.Calls = append(f.Calls, name)
	if f.FailNext != nil {
		err := f.FailNext
		f.FailNext = nil
		return err
	}
	return nil
}

func (f *LoadBalancerAPI) nextUUID(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%04d", prefix, f.seq)
}

// copyService applies StateOverride to a copy of the service.
func (f *LoadBalancerAPI) copyService(lb *upcloud.LoadBalancer) *upcloud.LoadBalancer {
	cp := *lb
	if f.StateOverride != "" {
		cp.OperationalState = f.StateOverride
	}
	return &cp
}

func (f *LoadBalancerAPI) copyBundle(b *upcloud.LoadBalancerCertificateBundle) *upcloud.LoadBalancerCertificateBundle {
	cp := *b
	if f.BundleStateOverride != "" {
		cp.OperationalState = f.BundleStateOverride
	}
	return &cp
}

func (f *LoadBalancerAPI) findBackend(lb *upcloud.LoadBalancer, name string) *upcloud.LoadBalancerBackend {
	for i := range lb.Backends {
		if lb.Backends[i].Name == name {
			return &lb.Backends[i]
		}
	}
	return nil
}

func (f *LoadBalancerAPI) findMember(be *upcloud.LoadBalancerBackend, name string) *upcloud.LoadBalancerBackendMember {
	for i := range be.Members {
		if be.Members[i].Name == name {
			return &be.Members[i]
		}
	}
	return nil
}

func (f *LoadBalancerAPI) findBackendTLS(be *upcloud.LoadBalancerBackend, name string) *upcloud.LoadBalancerBackendTLSConfig {
	for i := range be.TLSConfigs {
		if be.TLSConfigs[i].Name == name {
			return &be.TLSConfigs[i]
		}
	}
	return nil
}

func (f *LoadBalancerAPI) findResolver(lb *upcloud.LoadBalancer, name string) *upcloud.LoadBalancerResolver {
	for i := range lb.Resolvers {
		if lb.Resolvers[i].Name == name {
			return &lb.Resolvers[i]
		}
	}
	return nil
}

func (f *LoadBalancerAPI) findFrontend(lb *upcloud.LoadBalancer, name string) *upcloud.LoadBalancerFrontend {
	for i := range lb.Frontends {
		if lb.Frontends[i].Name == name {
			return &lb.Frontends[i]
		}
	}
	return nil
}

func (f *LoadBalancerAPI) findRule(fe *upcloud.LoadBalancerFrontend, name string) *upcloud.LoadBalancerFrontendRule {
	for i := range fe.Rules {
		if fe.Rules[i].Name == name {
			return &fe.Rules[i]
		}
	}
	return nil
}

func (f *LoadBalancerAPI) findFrontendTLS(fe *upcloud.LoadBalancerFrontend, name string) *upcloud.LoadBalancerFrontendTLSConfig {
	for i := range fe.TLSConfigs {
		if fe.TLSConfigs[i].Name == name {
			return &fe.TLSConfigs[i]
		}
	}
	return nil
}

// convBackend converts a request backend (and its nested members / TLS configs)
// to the stored upcloud shape.
func convBackend(r request.LoadBalancerBackend) upcloud.LoadBalancerBackend {
	be := upcloud.LoadBalancerBackend{
		Name:       r.Name,
		Resolver:   r.Resolver,
		Properties: r.Properties,
	}
	for _, m := range r.Members {
		be.Members = append(be.Members, convMember(m))
	}
	for _, c := range r.TLSConfigs {
		be.TLSConfigs = append(be.TLSConfigs, convBackendTLS(c))
	}
	return be
}

func convMember(r request.LoadBalancerBackendMember) upcloud.LoadBalancerBackendMember {
	return upcloud.LoadBalancerBackendMember{
		Name:        r.Name,
		IP:          r.IP,
		Port:        r.Port,
		Weight:      r.Weight,
		MaxSessions: r.MaxSessions,
		Type:        r.Type,
		Enabled:     r.Enabled,
	}
}

func convBackendTLS(r request.LoadBalancerBackendTLSConfig) upcloud.LoadBalancerBackendTLSConfig {
	return upcloud.LoadBalancerBackendTLSConfig{Name: r.Name, CertificateBundleUUID: r.CertificateBundleUUID}
}

func convResolver(r request.LoadBalancerResolver) upcloud.LoadBalancerResolver {
	return upcloud.LoadBalancerResolver{
		Name:         r.Name,
		Nameservers:  r.Nameservers,
		Retries:      r.Retries,
		Timeout:      r.Timeout,
		TimeoutRetry: r.TimeoutRetry,
		CacheValid:   r.CacheValid,
		CacheInvalid: r.CacheInvalid,
	}
}

func convFrontend(r request.LoadBalancerFrontend) upcloud.LoadBalancerFrontend {
	fe := upcloud.LoadBalancerFrontend{
		Name:           r.Name,
		Mode:           r.Mode,
		Port:           r.Port,
		Networks:       r.Networks,
		DefaultBackend: r.DefaultBackend,
		Rules:          convRules(r.Rules),
		TLSConfigs:     convFrontendTLS(r.TLSConfigs),
		Properties:     r.Properties,
	}
	return fe
}

func convRule(in request.LoadBalancerFrontendRule) upcloud.LoadBalancerFrontendRule {
	return upcloud.LoadBalancerFrontendRule{
		Name:              in.Name,
		Priority:          in.Priority,
		MatchingCondition: in.MatchingCondition,
		Matchers:          in.Matchers,
		Actions:           in.Actions,
	}
}

func convRules(in []request.LoadBalancerFrontendRule) []upcloud.LoadBalancerFrontendRule {
	out := make([]upcloud.LoadBalancerFrontendRule, 0, len(in))
	for _, r := range in {
		out = append(out, upcloud.LoadBalancerFrontendRule{
			Name:              r.Name,
			Priority:          r.Priority,
			MatchingCondition: r.MatchingCondition,
			Matchers:          r.Matchers,
			Actions:           r.Actions,
		})
	}
	return out
}

func convFrontendTLS(in []request.LoadBalancerFrontendTLSConfig) []upcloud.LoadBalancerFrontendTLSConfig {
	out := make([]upcloud.LoadBalancerFrontendTLSConfig, 0, len(in))
	for _, c := range in {
		out = append(out, upcloud.LoadBalancerFrontendTLSConfig{Name: c.Name, CertificateBundleUUID: c.CertificateBundleUUID})
	}
	return out
}

func convBackends(in []request.LoadBalancerBackend) []upcloud.LoadBalancerBackend {
	out := make([]upcloud.LoadBalancerBackend, 0, len(in))
	for _, b := range in {
		out = append(out, convBackend(b))
	}
	return out
}

func convResolvers(in []request.LoadBalancerResolver) []upcloud.LoadBalancerResolver {
	out := make([]upcloud.LoadBalancerResolver, 0, len(in))
	for _, r := range in {
		out = append(out, convResolver(r))
	}
	return out
}

func convFrontends(in []request.LoadBalancerFrontend) []upcloud.LoadBalancerFrontend {
	out := make([]upcloud.LoadBalancerFrontend, 0, len(in))
	for _, f := range in {
		out = append(out, convFrontend(f))
	}
	return out
}

// --- Service ---

// CreateLoadBalancer implements upcloudapi.LoadBalancerAPI. It assigns a UUID,
// sets OperationalState to running, and materializes the requested networks
// with a DNSName of "<name>.lb.upcloud.com" (public networks also get an IP).
func (f *LoadBalancerAPI) CreateLoadBalancer(ctx context.Context, r *request.CreateLoadBalancerRequest) (*upcloud.LoadBalancer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateLoadBalancer"); err != nil {
		return nil, err
	}
	networks := make([]upcloud.LoadBalancerNetwork, 0, len(r.Networks))
	for _, n := range r.Networks {
		nw := upcloud.LoadBalancerNetwork{UUID: n.UUID, Name: n.Name, Type: n.Type, Family: n.Family}
		if nw.UUID == "" {
			nw.UUID = f.nextUUID("lbnet")
		}
		nw.DNSName = n.Name + ".lb.upcloud.com"
		if n.Type == upcloud.LoadBalancerNetworkTypePublic {
			nw.IPAddresses = []upcloud.LoadBalancerIPAddress{{Address: fmt.Sprintf("185.70.%d.%d", f.seq%200, f.seq%250)}}
		}
		networks = append(networks, nw)
	}
	lb := &upcloud.LoadBalancer{
		UUID:             f.nextUUID("lb"),
		Name:             r.Name,
		Zone:             r.Zone,
		Plan:             r.Plan,
		Networks:         networks,
		Labels:           r.Labels,
		ConfiguredStatus: r.ConfiguredStatus,
		OperationalState: upcloud.LoadBalancerOperationalStateRunning,
		Frontends:        convFrontends(r.Frontends),
		Backends:         convBackends(r.Backends),
		Resolvers:        convResolvers(r.Resolvers),
		MaintenanceDOW:   r.MaintenanceDOW,
		MaintenanceTime:  r.MaintenanceTime,
	}
	f.LoadBalancers[lb.UUID] = lb
	return f.copyService(lb), nil
}

// GetLoadBalancers implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancers(ctx context.Context, r *request.GetLoadBalancersRequest) ([]upcloud.LoadBalancer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]upcloud.LoadBalancer, 0, len(f.LoadBalancers))
	for _, lb := range f.LoadBalancers {
		out = append(out, *lb)
	}
	return out, nil
}

// GetLoadBalancer implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancer(ctx context.Context, r *request.GetLoadBalancerRequest) (*upcloud.LoadBalancer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.UUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	return f.copyService(lb), nil
}

// ModifyLoadBalancer implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) ModifyLoadBalancer(ctx context.Context, r *request.ModifyLoadBalancerRequest) (*upcloud.LoadBalancer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyLoadBalancer"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.UUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	if r.Name != "" {
		lb.Name = r.Name
	}
	if r.Plan != "" {
		lb.Plan = r.Plan
	}
	if r.ConfiguredStatus != "" {
		lb.ConfiguredStatus = upcloud.LoadBalancerConfiguredStatus(r.ConfiguredStatus)
	}
	if r.Labels != nil {
		lb.Labels = *r.Labels
	}
	if r.MaintenanceDOW != "" {
		lb.MaintenanceDOW = r.MaintenanceDOW
	}
	if r.MaintenanceTime != "" {
		lb.MaintenanceTime = r.MaintenanceTime
	}
	return f.copyService(lb), nil
}

// DeleteLoadBalancer implements upcloudapi.LoadBalancerAPI. A service with
// frontends, backends, or resolvers cannot be deleted (409).
func (f *LoadBalancerAPI) DeleteLoadBalancer(ctx context.Context, r *request.DeleteLoadBalancerRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteLoadBalancer"); err != nil {
		return err
	}
	lb := f.LoadBalancers[r.UUID]
	if lb == nil {
		return NotFound("load balancer")
	}
	if len(lb.Frontends) > 0 || len(lb.Backends) > 0 || len(lb.Resolvers) > 0 {
		return Conflict("load balancer still has frontends, backends, or resolvers")
	}
	delete(f.LoadBalancers, r.UUID)
	return nil
}

// --- Backends ---

// CreateLoadBalancerBackend implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) CreateLoadBalancerBackend(ctx context.Context, r *request.CreateLoadBalancerBackendRequest) (*upcloud.LoadBalancerBackend, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateLoadBalancerBackend"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	if f.findBackend(lb, r.Backend.Name) != nil {
		return nil, Conflict("backend already exists")
	}
	be := convBackend(r.Backend)
	be.CreatedAt = time.Now()
	lb.Backends = append(lb.Backends, be)
	return &lb.Backends[len(lb.Backends)-1], nil
}

// GetLoadBalancerBackends implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerBackends(ctx context.Context, r *request.GetLoadBalancerBackendsRequest) ([]upcloud.LoadBalancerBackend, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	return lb.Backends, nil
}

// GetLoadBalancerBackend implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerBackend(ctx context.Context, r *request.GetLoadBalancerBackendRequest) (*upcloud.LoadBalancerBackend, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	be := f.findBackend(lb, r.Name)
	if be == nil {
		return nil, NotFound("load balancer backend")
	}
	return be, nil
}

// ModifyLoadBalancerBackend implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) ModifyLoadBalancerBackend(ctx context.Context, r *request.ModifyLoadBalancerBackendRequest) (*upcloud.LoadBalancerBackend, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyLoadBalancerBackend"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	be := f.findBackend(lb, r.Name)
	if be == nil {
		return nil, NotFound("load balancer backend")
	}
	if r.Backend.Resolver != nil {
		be.Resolver = *r.Backend.Resolver
	}
	if r.Backend.Properties != nil {
		be.Properties = r.Backend.Properties
	}
	return be, nil
}

// DeleteLoadBalancerBackend implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) DeleteLoadBalancerBackend(ctx context.Context, r *request.DeleteLoadBalancerBackendRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteLoadBalancerBackend"); err != nil {
		return err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return NotFound("load balancer")
	}
	for i := range lb.Backends {
		if lb.Backends[i].Name == r.Name {
			lb.Backends = append(lb.Backends[:i], lb.Backends[i+1:]...)
			return nil
		}
	}
	return NotFound("load balancer backend")
}

// --- Backend members ---

// CreateLoadBalancerBackendMember implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) CreateLoadBalancerBackendMember(ctx context.Context, r *request.CreateLoadBalancerBackendMemberRequest) (*upcloud.LoadBalancerBackendMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateLoadBalancerBackendMember"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	be := f.findBackend(lb, r.BackendName)
	if be == nil {
		return nil, NotFound("load balancer backend")
	}
	if f.findMember(be, r.Member.Name) != nil {
		return nil, Conflict("backend member already exists")
	}
	m := convMember(r.Member)
	m.CreatedAt = time.Now()
	be.Members = append(be.Members, m)
	return &be.Members[len(be.Members)-1], nil
}

// GetLoadBalancerBackendMembers implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerBackendMembers(ctx context.Context, r *request.GetLoadBalancerBackendMembersRequest) ([]upcloud.LoadBalancerBackendMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	be := f.findBackend(lb, r.BackendName)
	if be == nil {
		return nil, NotFound("load balancer backend")
	}
	return be.Members, nil
}

// GetLoadBalancerBackendMember implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerBackendMember(ctx context.Context, r *request.GetLoadBalancerBackendMemberRequest) (*upcloud.LoadBalancerBackendMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	be := f.findBackend(lb, r.BackendName)
	if be == nil {
		return nil, NotFound("load balancer backend")
	}
	m := f.findMember(be, r.Name)
	if m == nil {
		return nil, NotFound("load balancer backend member")
	}
	return m, nil
}

// ModifyLoadBalancerBackendMember implements upcloudapi.LoadBalancerAPI. Only
// the non-nil pointer fields are applied.
func (f *LoadBalancerAPI) ModifyLoadBalancerBackendMember(ctx context.Context, r *request.ModifyLoadBalancerBackendMemberRequest) (*upcloud.LoadBalancerBackendMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyLoadBalancerBackendMember"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	be := f.findBackend(lb, r.BackendName)
	if be == nil {
		return nil, NotFound("load balancer backend")
	}
	m := f.findMember(be, r.Name)
	if m == nil {
		return nil, NotFound("load balancer backend member")
	}
	if r.Member.Type != "" {
		m.Type = r.Member.Type
	}
	if r.Member.Name != "" {
		m.Name = r.Member.Name
	}
	if r.Member.Weight != nil {
		m.Weight = *r.Member.Weight
	}
	if r.Member.MaxSessions != nil {
		m.MaxSessions = *r.Member.MaxSessions
	}
	if r.Member.Enabled != nil {
		m.Enabled = *r.Member.Enabled
	}
	if r.Member.IP != nil {
		m.IP = *r.Member.IP
	}
	if r.Member.Port != 0 {
		m.Port = r.Member.Port
	}
	return m, nil
}

// DeleteLoadBalancerBackendMember implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) DeleteLoadBalancerBackendMember(ctx context.Context, r *request.DeleteLoadBalancerBackendMemberRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteLoadBalancerBackendMember"); err != nil {
		return err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return NotFound("load balancer")
	}
	be := f.findBackend(lb, r.BackendName)
	if be == nil {
		return NotFound("load balancer backend")
	}
	for i := range be.Members {
		if be.Members[i].Name == r.Name {
			be.Members = append(be.Members[:i], be.Members[i+1:]...)
			return nil
		}
	}
	return NotFound("load balancer backend member")
}

// --- Backend TLS configs ---

// CreateLoadBalancerBackendTLSConfig implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) CreateLoadBalancerBackendTLSConfig(ctx context.Context, r *request.CreateLoadBalancerBackendTLSConfigRequest) (*upcloud.LoadBalancerBackendTLSConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateLoadBalancerBackendTLSConfig"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	be := f.findBackend(lb, r.BackendName)
	if be == nil {
		return nil, NotFound("load balancer backend")
	}
	if f.findBackendTLS(be, r.Config.Name) != nil {
		return nil, Conflict("backend TLS config already exists")
	}
	c := convBackendTLS(r.Config)
	c.CreatedAt = time.Now()
	be.TLSConfigs = append(be.TLSConfigs, c)
	return &be.TLSConfigs[len(be.TLSConfigs)-1], nil
}

// GetLoadBalancerBackendTLSConfigs implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerBackendTLSConfigs(ctx context.Context, r *request.GetLoadBalancerBackendTLSConfigsRequest) ([]upcloud.LoadBalancerBackendTLSConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	be := f.findBackend(lb, r.BackendName)
	if be == nil {
		return nil, NotFound("load balancer backend")
	}
	return be.TLSConfigs, nil
}

// GetLoadBalancerBackendTLSConfig implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerBackendTLSConfig(ctx context.Context, r *request.GetLoadBalancerBackendTLSConfigRequest) (*upcloud.LoadBalancerBackendTLSConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	be := f.findBackend(lb, r.BackendName)
	if be == nil {
		return nil, NotFound("load balancer backend")
	}
	c := f.findBackendTLS(be, r.Name)
	if c == nil {
		return nil, NotFound("load balancer backend TLS config")
	}
	return c, nil
}

// ModifyLoadBalancerBackendTLSConfig implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) ModifyLoadBalancerBackendTLSConfig(ctx context.Context, r *request.ModifyLoadBalancerBackendTLSConfigRequest) (*upcloud.LoadBalancerBackendTLSConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyLoadBalancerBackendTLSConfig"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	be := f.findBackend(lb, r.BackendName)
	if be == nil {
		return nil, NotFound("load balancer backend")
	}
	c := f.findBackendTLS(be, r.Name)
	if c == nil {
		return nil, NotFound("load balancer backend TLS config")
	}
	c.CertificateBundleUUID = r.Config.CertificateBundleUUID
	return c, nil
}

// DeleteLoadBalancerBackendTLSConfig implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) DeleteLoadBalancerBackendTLSConfig(ctx context.Context, r *request.DeleteLoadBalancerBackendTLSConfigRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteLoadBalancerBackendTLSConfig"); err != nil {
		return err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return NotFound("load balancer")
	}
	be := f.findBackend(lb, r.BackendName)
	if be == nil {
		return NotFound("load balancer backend")
	}
	for i := range be.TLSConfigs {
		if be.TLSConfigs[i].Name == r.Name {
			be.TLSConfigs = append(be.TLSConfigs[:i], be.TLSConfigs[i+1:]...)
			return nil
		}
	}
	return NotFound("load balancer backend TLS config")
}

// --- Resolvers ---

// CreateLoadBalancerResolver implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) CreateLoadBalancerResolver(ctx context.Context, r *request.CreateLoadBalancerResolverRequest) (*upcloud.LoadBalancerResolver, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateLoadBalancerResolver"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	if f.findResolver(lb, r.Resolver.Name) != nil {
		return nil, Conflict("resolver already exists")
	}
	res := convResolver(r.Resolver)
	res.CreatedAt = time.Now()
	lb.Resolvers = append(lb.Resolvers, res)
	return &lb.Resolvers[len(lb.Resolvers)-1], nil
}

// GetLoadBalancerResolvers implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerResolvers(ctx context.Context, r *request.GetLoadBalancerResolversRequest) ([]upcloud.LoadBalancerResolver, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	return lb.Resolvers, nil
}

// GetLoadBalancerResolver implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerResolver(ctx context.Context, r *request.GetLoadBalancerResolverRequest) (*upcloud.LoadBalancerResolver, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	res := f.findResolver(lb, r.Name)
	if res == nil {
		return nil, NotFound("load balancer resolver")
	}
	return res, nil
}

// ModifyLoadBalancerResolver implements upcloudapi.LoadBalancerAPI. The full
// resolver is sent, so all fields are replaced.
func (f *LoadBalancerAPI) ModifyLoadBalancerResolver(ctx context.Context, r *request.ModifyLoadBalancerResolverRequest) (*upcloud.LoadBalancerResolver, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyLoadBalancerResolver"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	for i := range lb.Resolvers {
		if lb.Resolvers[i].Name == r.Name {
			replaced := convResolver(r.Resolver)
			replaced.CreatedAt = lb.Resolvers[i].CreatedAt
			lb.Resolvers[i] = replaced
			return &lb.Resolvers[i], nil
		}
	}
	return nil, NotFound("load balancer resolver")
}

// DeleteLoadBalancerResolver implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) DeleteLoadBalancerResolver(ctx context.Context, r *request.DeleteLoadBalancerResolverRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteLoadBalancerResolver"); err != nil {
		return err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return NotFound("load balancer")
	}
	for i := range lb.Resolvers {
		if lb.Resolvers[i].Name == r.Name {
			lb.Resolvers = append(lb.Resolvers[:i], lb.Resolvers[i+1:]...)
			return nil
		}
	}
	return NotFound("load balancer resolver")
}

// --- Frontends ---

// CreateLoadBalancerFrontend implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) CreateLoadBalancerFrontend(ctx context.Context, r *request.CreateLoadBalancerFrontendRequest) (*upcloud.LoadBalancerFrontend, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateLoadBalancerFrontend"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	if f.findFrontend(lb, r.Frontend.Name) != nil {
		return nil, Conflict("frontend already exists")
	}
	fe := convFrontend(r.Frontend)
	fe.CreatedAt = time.Now()
	lb.Frontends = append(lb.Frontends, fe)
	return &lb.Frontends[len(lb.Frontends)-1], nil
}

// GetLoadBalancerFrontends implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerFrontends(ctx context.Context, r *request.GetLoadBalancerFrontendsRequest) ([]upcloud.LoadBalancerFrontend, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	return lb.Frontends, nil
}

// GetLoadBalancerFrontend implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerFrontend(ctx context.Context, r *request.GetLoadBalancerFrontendRequest) (*upcloud.LoadBalancerFrontend, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	fe := f.findFrontend(lb, r.Name)
	if fe == nil {
		return nil, NotFound("load balancer frontend")
	}
	return fe, nil
}

// ModifyLoadBalancerFrontend implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) ModifyLoadBalancerFrontend(ctx context.Context, r *request.ModifyLoadBalancerFrontendRequest) (*upcloud.LoadBalancerFrontend, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyLoadBalancerFrontend"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	fe := f.findFrontend(lb, r.Name)
	if fe == nil {
		return nil, NotFound("load balancer frontend")
	}
	if r.Frontend.Mode != "" {
		fe.Mode = r.Frontend.Mode
	}
	if r.Frontend.Port != 0 {
		fe.Port = r.Frontend.Port
	}
	if r.Frontend.DefaultBackend != "" {
		fe.DefaultBackend = r.Frontend.DefaultBackend
	}
	if r.Frontend.Properties != nil {
		fe.Properties = r.Frontend.Properties
	}
	if len(r.Frontend.Networks) > 0 {
		fe.Networks = r.Frontend.Networks
	}
	return fe, nil
}

// DeleteLoadBalancerFrontend implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) DeleteLoadBalancerFrontend(ctx context.Context, r *request.DeleteLoadBalancerFrontendRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteLoadBalancerFrontend"); err != nil {
		return err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return NotFound("load balancer")
	}
	for i := range lb.Frontends {
		if lb.Frontends[i].Name == r.Name {
			lb.Frontends = append(lb.Frontends[:i], lb.Frontends[i+1:]...)
			return nil
		}
	}
	return NotFound("load balancer frontend")
}

// --- Frontend rules ---

// CreateLoadBalancerFrontendRule implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) CreateLoadBalancerFrontendRule(ctx context.Context, r *request.CreateLoadBalancerFrontendRuleRequest) (*upcloud.LoadBalancerFrontendRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateLoadBalancerFrontendRule"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	fe := f.findFrontend(lb, r.FrontendName)
	if fe == nil {
		return nil, NotFound("load balancer frontend")
	}
	if f.findRule(fe, r.Rule.Name) != nil {
		return nil, Conflict("frontend rule already exists")
	}
	rule := convRule(r.Rule)
	rule.CreatedAt = time.Now()
	fe.Rules = append(fe.Rules, rule)
	return &fe.Rules[len(fe.Rules)-1], nil
}

// GetLoadBalancerFrontendRules implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerFrontendRules(ctx context.Context, r *request.GetLoadBalancerFrontendRulesRequest) ([]upcloud.LoadBalancerFrontendRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	fe := f.findFrontend(lb, r.FrontendName)
	if fe == nil {
		return nil, NotFound("load balancer frontend")
	}
	return fe.Rules, nil
}

// GetLoadBalancerFrontendRule implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerFrontendRule(ctx context.Context, r *request.GetLoadBalancerFrontendRuleRequest) (*upcloud.LoadBalancerFrontendRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	fe := f.findFrontend(lb, r.FrontendName)
	if fe == nil {
		return nil, NotFound("load balancer frontend")
	}
	rule := f.findRule(fe, r.Name)
	if rule == nil {
		return nil, NotFound("load balancer frontend rule")
	}
	return rule, nil
}

// ReplaceLoadBalancerFrontendRule implements upcloudapi.LoadBalancerAPI. The
// full rule replaces the existing one by name (created if absent).
func (f *LoadBalancerAPI) ReplaceLoadBalancerFrontendRule(ctx context.Context, r *request.ReplaceLoadBalancerFrontendRuleRequest) (*upcloud.LoadBalancerFrontendRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ReplaceLoadBalancerFrontendRule"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	fe := f.findFrontend(lb, r.FrontendName)
	if fe == nil {
		return nil, NotFound("load balancer frontend")
	}
	rule := convRule(r.Rule)
	rule.UpdatedAt = time.Now()
	for i := range fe.Rules {
		if fe.Rules[i].Name == rule.Name {
			fe.Rules[i] = rule
			return &fe.Rules[i], nil
		}
	}
	fe.Rules = append(fe.Rules, rule)
	return &fe.Rules[len(fe.Rules)-1], nil
}

// DeleteLoadBalancerFrontendRule implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) DeleteLoadBalancerFrontendRule(ctx context.Context, r *request.DeleteLoadBalancerFrontendRuleRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteLoadBalancerFrontendRule"); err != nil {
		return err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return NotFound("load balancer")
	}
	fe := f.findFrontend(lb, r.FrontendName)
	if fe == nil {
		return NotFound("load balancer frontend")
	}
	for i := range fe.Rules {
		if fe.Rules[i].Name == r.Name {
			fe.Rules = append(fe.Rules[:i], fe.Rules[i+1:]...)
			return nil
		}
	}
	return NotFound("load balancer frontend rule")
}

// --- Frontend TLS configs ---

// CreateLoadBalancerFrontendTLSConfig implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) CreateLoadBalancerFrontendTLSConfig(ctx context.Context, r *request.CreateLoadBalancerFrontendTLSConfigRequest) (*upcloud.LoadBalancerFrontendTLSConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateLoadBalancerFrontendTLSConfig"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	fe := f.findFrontend(lb, r.FrontendName)
	if fe == nil {
		return nil, NotFound("load balancer frontend")
	}
	if f.findFrontendTLS(fe, r.Config.Name) != nil {
		return nil, Conflict("frontend TLS config already exists")
	}
	c := upcloud.LoadBalancerFrontendTLSConfig{Name: r.Config.Name, CertificateBundleUUID: r.Config.CertificateBundleUUID}
	c.CreatedAt = time.Now()
	fe.TLSConfigs = append(fe.TLSConfigs, c)
	return &fe.TLSConfigs[len(fe.TLSConfigs)-1], nil
}

// GetLoadBalancerFrontendTLSConfigs implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerFrontendTLSConfigs(ctx context.Context, r *request.GetLoadBalancerFrontendTLSConfigsRequest) ([]upcloud.LoadBalancerFrontendTLSConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	fe := f.findFrontend(lb, r.FrontendName)
	if fe == nil {
		return nil, NotFound("load balancer frontend")
	}
	return fe.TLSConfigs, nil
}

// GetLoadBalancerFrontendTLSConfig implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerFrontendTLSConfig(ctx context.Context, r *request.GetLoadBalancerFrontendTLSConfigRequest) (*upcloud.LoadBalancerFrontendTLSConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	fe := f.findFrontend(lb, r.FrontendName)
	if fe == nil {
		return nil, NotFound("load balancer frontend")
	}
	c := f.findFrontendTLS(fe, r.Name)
	if c == nil {
		return nil, NotFound("load balancer frontend TLS config")
	}
	return c, nil
}

// ModifyLoadBalancerFrontendTLSConfig implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) ModifyLoadBalancerFrontendTLSConfig(ctx context.Context, r *request.ModifyLoadBalancerFrontendTLSConfigRequest) (*upcloud.LoadBalancerFrontendTLSConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyLoadBalancerFrontendTLSConfig"); err != nil {
		return nil, err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return nil, NotFound("load balancer")
	}
	fe := f.findFrontend(lb, r.FrontendName)
	if fe == nil {
		return nil, NotFound("load balancer frontend")
	}
	c := f.findFrontendTLS(fe, r.Name)
	if c == nil {
		return nil, NotFound("load balancer frontend TLS config")
	}
	c.CertificateBundleUUID = r.Config.CertificateBundleUUID
	return c, nil
}

// DeleteLoadBalancerFrontendTLSConfig implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) DeleteLoadBalancerFrontendTLSConfig(ctx context.Context, r *request.DeleteLoadBalancerFrontendTLSConfigRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteLoadBalancerFrontendTLSConfig"); err != nil {
		return err
	}
	lb := f.LoadBalancers[r.ServiceUUID]
	if lb == nil {
		return NotFound("load balancer")
	}
	fe := f.findFrontend(lb, r.FrontendName)
	if fe == nil {
		return NotFound("load balancer frontend")
	}
	for i := range fe.TLSConfigs {
		if fe.TLSConfigs[i].Name == r.Name {
			fe.TLSConfigs = append(fe.TLSConfigs[:i], fe.TLSConfigs[i+1:]...)
			return nil
		}
	}
	return NotFound("load balancer frontend TLS config")
}

// --- Certificate bundles ---

// CreateLoadBalancerCertificateBundle implements upcloudapi.LoadBalancerAPI.
// A manual/authority bundle starts idle; a dynamic bundle starts pending (the
// fake does not model challenge completion).
func (f *LoadBalancerAPI) CreateLoadBalancerCertificateBundle(ctx context.Context, r *request.CreateLoadBalancerCertificateBundleRequest) (*upcloud.LoadBalancerCertificateBundle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("CreateLoadBalancerCertificateBundle"); err != nil {
		return nil, err
	}
	state := upcloud.LoadBalancerCertificateBundleOperationalStateIdle
	if r.Type == upcloud.LoadBalancerCertificateBundleTypeDynamic {
		state = upcloud.LoadBalancerCertificateBundleOperationalStatePending
	}
	bundle := &upcloud.LoadBalancerCertificateBundle{
		UUID:             f.nextUUID("cb"),
		Name:             r.Name,
		Type:             r.Type,
		Certificate:      r.Certificate,
		Intermediates:    r.Intermediates,
		Hostnames:        r.Hostnames,
		KeyType:          r.KeyType,
		OperationalState: state,
		NotBefore:        time.Now().Add(-1 * time.Hour),
		NotAfter:         time.Now().Add(365 * 24 * time.Hour),
	}
	f.CertificateBundles[bundle.UUID] = bundle
	return f.copyBundle(bundle), nil
}

// GetLoadBalancerCertificateBundles implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerCertificateBundles(ctx context.Context, r *request.GetLoadBalancerCertificateBundlesRequest) ([]upcloud.LoadBalancerCertificateBundle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]upcloud.LoadBalancerCertificateBundle, 0, len(f.CertificateBundles))
	for _, b := range f.CertificateBundles {
		out = append(out, *b)
	}
	return out, nil
}

// GetLoadBalancerCertificateBundle implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) GetLoadBalancerCertificateBundle(ctx context.Context, r *request.GetLoadBalancerCertificateBundleRequest) (*upcloud.LoadBalancerCertificateBundle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b := f.CertificateBundles[r.UUID]
	if b == nil {
		return nil, NotFound("certificate bundle")
	}
	return f.copyBundle(b), nil
}

// ModifyLoadBalancerCertificateBundle implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) ModifyLoadBalancerCertificateBundle(ctx context.Context, r *request.ModifyLoadBalancerCertificateBundleRequest) (*upcloud.LoadBalancerCertificateBundle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("ModifyLoadBalancerCertificateBundle"); err != nil {
		return nil, err
	}
	b := f.CertificateBundles[r.UUID]
	if b == nil {
		return nil, NotFound("certificate bundle")
	}
	if r.Name != "" {
		b.Name = r.Name
	}
	if r.Certificate != "" {
		b.Certificate = r.Certificate
	}
	if r.Intermediates != nil {
		b.Intermediates = *r.Intermediates
	}
	if len(r.Hostnames) > 0 {
		b.Hostnames = r.Hostnames
	}
	// A key/cert update resets a dynamic bundle to pending; manual bundles
	// stay idle.
	if b.Type == upcloud.LoadBalancerCertificateBundleTypeDynamic && r.PrivateKey != "" {
		b.OperationalState = upcloud.LoadBalancerCertificateBundleOperationalStatePending
	}
	return f.copyBundle(b), nil
}

// DeleteLoadBalancerCertificateBundle implements upcloudapi.LoadBalancerAPI.
func (f *LoadBalancerAPI) DeleteLoadBalancerCertificateBundle(ctx context.Context, r *request.DeleteLoadBalancerCertificateBundleRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.record("DeleteLoadBalancerCertificateBundle"); err != nil {
		return err
	}
	if _, ok := f.CertificateBundles[r.UUID]; !ok {
		return NotFound("certificate bundle")
	}
	delete(f.CertificateBundles, r.UUID)
	return nil
}
