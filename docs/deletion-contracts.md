# Adapter deletion contracts

All 25 adapters are covered against the in-memory UpCloud fake API.
The tests run in `make test` and `mise run check`; no live UpCloud account
or paid e2e dispatch is required.

Each row covers an empty external status identity without API calls,
already-absent success, conflict/pending deletion through `errors.Is`
against `reconciler.ErrPending`, and propagation of unexpected errors.
Explicit primary-user and database termination-protection errors remain
actionable errors rather than being hidden as ordinary dependency waits.

The table lists the test entrypoints. Table-driven entrypoints contain
the individual contract subtests.

| Group | Kind | Test entrypoints |
|---|---|---|
| database | ManagedDatabase | `TestManagedDatabaseDeleteAbsentResourceSucceeds`<br>`TestManagedDatabaseDeleteConflictReturnsPending`<br>`TestManagedDatabaseDeleteEmptyIdentityMakesNoAPICalls`<br>`TestManagedDatabaseDeleteUnexpectedErrorRemainsError` |
| database | ManagedDatabaseLogicalDatabase | `TestManagedDatabaseLogicalDatabaseDeleteAbsentResourceSucceeds`<br>`TestManagedDatabaseLogicalDatabaseDeleteConflictReturnsPending`<br>`TestManagedDatabaseLogicalDatabaseDeleteEmptyIdentityMakesNoAPICalls`<br>`TestManagedDatabaseLogicalDatabaseDeleteUnexpectedErrorRemainsError` |
| database | ManagedDatabaseUser | `TestManagedDatabaseUserDeleteAbsentResourceSucceeds`<br>`TestManagedDatabaseUserDeleteConflictReturnsPending`<br>`TestManagedDatabaseUserDeleteEmptyIdentityMakesNoAPICalls`<br>`TestManagedDatabaseUserDeleteUnexpectedErrorRemainsError` |
| loadbalancer | LoadBalancer | `TestLoadBalancerDeleteBlockedByChildren`<br>`TestLoadBalancerDeleteContract` |
| loadbalancer | LoadBalancerBackend | `TestLoadBalancerBackendDeleteContract` |
| loadbalancer | LoadBalancerBackendMember | `TestLoadBalancerBackendMemberDeleteContract` |
| loadbalancer | LoadBalancerBackendTLSConfig | `TestLoadBalancerBackendTLSConfigDeleteContract` |
| loadbalancer | LoadBalancerCertificateBundle | `TestLoadBalancerCertificateBundleDeleteContract` |
| loadbalancer | LoadBalancerFrontend | `TestLoadBalancerFrontendDeleteContract` |
| loadbalancer | LoadBalancerFrontendRule | `TestLoadBalancerFrontendRuleDeleteContract` |
| loadbalancer | LoadBalancerFrontendTLSConfig | `TestLoadBalancerFrontendTLSConfigDeleteContract` |
| loadbalancer | LoadBalancerResolver | `TestLoadBalancerResolverDeleteContract` |
| network | FloatingIP | `TestFloatingIPDeleteContract` |
| network | Gateway | `TestGatewayDeleteContract` |
| network | GatewayConnection | `TestGatewayConnectionDeleteContract` |
| network | GatewayTunnel | `TestGatewayTunnelDeleteContract` |
| network | Network | `TestNetworkDeleteContract` |
| network | NetworkPeering | `TestNetworkPeeringDeleteContract` |
| network | Router | `TestRouterDeleteContract` |
| objectstorage | ManagedObjectStorage | `TestManagedObjectStorageDeleteContract` |
| objectstorage | ObjectStorageAccessKey | `TestObjectStorageAccessKeyDeleteContract` |
| objectstorage | ObjectStorageBucket | `TestObjectStorageBucketDeleteContract` |
| objectstorage | ObjectStorageCustomDomain | `TestObjectStorageCustomDomainDeleteContract` |
| objectstorage | ObjectStoragePolicy | `TestObjectStoragePolicyDeleteContract` |
| objectstorage | ObjectStorageUser | `TestObjectStorageUserDeleteContract` |
