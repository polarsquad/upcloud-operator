# AGENTS.md: upcloud-operator

Guidance for AI coding agents working in this repository.

## What this repo is

A kubebuilder (go/v4, multigroup) operator that manages UpCloud hosted
products through namespaced CRDs. One generic reconciler drives every kind;
each kind contributes an adapter that maps its spec onto the UpCloud API.

- `api/<group>/v1alpha1/`: CRD types per API group (`network`, `database`,
  `objectstorage`, `loadbalancer`). `api/common/`: shared spec fragments.
- `internal/reconciler/`: the generic `Reconciler[T]`, `Adapter[T]`
  interface, condition helpers, sentinel errors.
- `internal/upcloudapi/`: SDK client construction, narrow per-group API
  interfaces, label and error helpers. `internal/upcloudapi/fake/`:
  in-memory fakes used by adapter unit tests.
- `internal/resolve/`: cross-CR reference resolution (parent UUIDs,
  Network and Router attachments, Secret keys).
- `internal/controller/<group>/`: one `<kind>_controller.go` (RBAC markers
  and manager wiring), `<kind>_adapter.go`, `<kind>_adapter_test.go`.
- `config/`: kustomize install manifests; `config/samples/` has one example
  per kind and is applied by the e2e test.
- `test/e2e/`: kind-based smoke test (no UpCloud calls).
  `test/e2e-upcloud/`: real-API test, manual dispatch only.

## The golden rules (read before changing anything)

1. Never commit to `main`; branch, PR, green CI, squash merge.
2. Every adapter change ships with a unit test against the fake API. No
   test may call the real UpCloud API.
3. Generated files (`zz_generated.deepcopy.go`, `config/crd/bases`,
   `config/rbac/role.yaml`) are regenerated with `make manifests generate`,
   never edited by hand.
4. Secrets: credentials live only in the `upcloud-credentials` Secret.
   Connection strings, passwords and access keys the operator produces are
   written to Secrets owned by the CR, never to `status`.
5. Run `make manifests generate fmt vet lint test` before pushing.
6. No em-dashes in any committed document.

## Adapter contract

`Observe` must be side-effect free on UpCloud, may update `status` fields
(not conditions) and must adopt an existing resource by the `services.k8s.upcloud/uid` label
when `status.uuid` is empty. `Create` sets the external identifier on the
object. `Update` returns `reconciler.ErrPending` when the external resource
is mid-transition. `Delete` is idempotent and returns `reconciler.ErrPending`
until the resource is gone.

## Validation

```sh
mise run check                  # manifests generate fmt vet lint test
make test-e2e                   # kind cluster smoke test
kubectl kustomize config/default
```

## Where to look next

- `docs/resources.md`: every kind, its UpCloud counterpart, produced Secrets.
- `docs/development.md`: adding a kind step by step.
