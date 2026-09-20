# Development

This guide covers the local loop, how to add a kind, and how to cut a
release of UCK (UpCloud Controllers for Kubernetes). The project was
previously called `upcloud-operator`; see the
[migration guide](migration-to-uck.md) for identifier changes and upgrade
steps. Go imports and the repository URL retain the old path until the
GitHub repository is renamed.

## The local loop

```sh
mise install          # once: installs the pinned Go, controller-gen, etc.
mise run check        # = make manifests generate fmt vet lint test
```

`make run` runs the manager against your current kube context. Point it
at a kind cluster and export a token first:

```sh
kind create cluster --name upcloud-dev
kind load docker-image <image> --name upcloud-dev  # only for make deploy
export UPCLOUD_TOKEN=***
make install                 # CRDs into the current context
make run                     # or: make deploy IMG=<image>
```

To build the installer locally:

```sh
make build-installer IMG=uck:dev
kubectl apply -f dist/install.yaml
```

## Parent-first deletion regression tests

```sh
go test ./internal/controller/integration -run TestParentFirst -count=1 -v
go test -race ./internal/controller/integration -count=10
```

These fake-API integration tests exercise `Reconciler.Reconcile`, the real
adapters and the controllers' finalizer constants. They create both CRs through
the Kubernetes fake client, reconcile them to Ready, then request parent deletion
before child deletion. Status subresources and finalizer-driven CR removal are
enabled. Each pending pass must return a retry interval, retain its identity and
finalizer, and report `Ready=False/Deleting`. Exact cloud identities are checked
through the fake APIs before and after cleanup; tests never delete cloud maps or
remove finalizers themselves.

| Test | Observed semantics |
| --- | --- |
| `TestParentFirstRouterNetworkRetriesAttachedNetwork` | An attached network blocks the fake router's delete with 409. Two pending parent passes retain both resources; deleting the network through its controller unblocks the router retry. |
| `TestParentFirstObjectStorageBucketRetriesUntilEmpty` | With `force=false`, the bucket blocks service deletion. Bucket and service accepted deletes each retain their CR finalizer until a later pass confirms absence. The bucket CR drains even after the parent CR is gone. |
| `TestParentFirstDatabaseUserConvergesAfterCascade` | Parent deletion enters `delete-service`, retaining its finalizer. A later parent retry completes the fake deletion and cascades its users. A controlled, one-shot user-delete 409 exercises the child retry, then the saved service identity permits a 404 cleanup after the parent CR is gone. This injected conflict is not a vendor dependency claim. |

The dependency direction follows the adapters and fakes, checked against vendor
API documentation: [router deletion](https://developers.upcloud.com/1.3/13-networks/#delete-router)
lists attached networks as a blocker; [MOS service deletion](https://developers.upcloud.com/1.3/21-managed-object-storage/#delete-service)
requires removal of buckets and IAM entities unless forced;
[database deletion](https://developers.upcloud.com/1.3/16-managed-database/#delete-managed-database)
erases the service and its data rather than requiring user CR deletion first.
The router documentation calls `ROUTER_ATTACHED` "400 Conflict", while the fake
uses 409 and `RouterAdapter.Delete` only maps 409 to `ErrPending`. The test proves
the fake's attachment/retry contract, not the vendor's exact status mapping.
That discrepancy needs vendor confirmation, not an invented reverse dependency.

Scheduling is deterministic and single-threaded, not a controller-manager
workqueue or an API-server/watch test. The tests assert the requested requeue
before delivering the next pending pass. Cloud observations use locking API
methods; synchronous `Calls`/`FailNext` access has no background manager to race.
The database fake advances deletion on its next delete call; the MOS fake removes
accepted deletes immediately, but the adapters still require confirmation. Real
cloud latency and error mapping remain confirmation work for a separately
approved paid e2e run, not prerequisites for these regression tests.

Mutation checks must fail if pending requeues are suppressed, pending finalizers
are dropped, completed finalizers are retained, cloud deletion is skipped,
conflicts lose their pending classification, accepted asynchronous deletes count
as gone, or a cascaded database user's 404 is rejected. Restore each mutation
before running the full gate. A tests-only change can use those deliberate
regressions for RED, then the restored implementation for GREEN.

## Adding a kind

Worked example: adding a kind to an existing group. Every step has a
pre-existing counterpart to copy: the closest analogue is usually in
the same group.

1. **Scaffold.** kubebuilder's multigroup layout. Create the types file
   `api/<group>/v1alpha1/<kind>_types.go` with the Spec, Status, List and
   the `+kubebuilder:object:root=true` / `+kubebuilder:subresource:status`
   markers, plus the `Register` func. Copy an existing types file in the
   same group as the template.

2. **Types checklist.**
   - `spec.deletionPolicy` (`common.DeletionPolicy`, default Delete).
   - Immutable fields get
     `+kubebuilder:validation:XValidation:rule="self == oldSelf"`.
   - `status.conditions []metav1.Condition` plus the identity field
     (`uuid`, `name`, `username`, ...) the adapter writes.
   - `api/<group>/v1alpha1/groupversion_info.go` already registers the
     group; the new type needs `Register` added there (or its own
     `init`).
   - Run `make manifests generate`; never edit the generated files.

3. **Adapter.** `internal/controller/<group>/<kind>_adapter.go`
   implementing `reconciler.Adapter[*<kind>]`:
   - `Observe`: side-effect free on UpCloud. Adopt by the `k8s-uid`
     label where the API has labels, by `(parent, name)` where it does
     not. Write `status` identity fields here.
   - `Create`: set the external identifier on the object on return.
   - `Update`: push drifted fields; return `reconciler.ErrPending` when
     the resource is mid-transition.
   - `Delete`: idempotent; return `reconciler.ErrPending` until the
     resource is actually gone (404).
   - Secrets the operator produces are written with
     `internal/k8s` helpers, owned by the CR, never put in `status`.

4. **Fake.** `internal/upcloudapi/fake/<group>.go`: add the new methods
   to the group's in-memory fake with the same locking and `Calls` /
   `StateOverride` / `FailNext` behavior as its siblings. The real
   `*service.Service` already satisfies the group interface; the fake
   only needs to be behaviorally honest (409s, 404s, label filters).
   Add a validation to the fake when the real API rejected a value a
   test could not catch: the object storage fake rejects policy
   `Resource` values in the invented `arn:upcloud:` namespace with 400,
   and a test applies the policy sample through it.

5. **Narrow interface.** If the kind needs a method the group interface
   does not have, add it to `internal/upcloudapi/<group>.go` and keep
   `var _ <Group>API = (*service.Service)(nil)`.

6. **Tests.** `internal/controller/<group>/<kind>_adapter_test.go`
   against the fake. Minimum coverage: create (and defaults), adoption
   where possible, drift plus update, pending / transitional handling,
   delete idempotency, parent-missing behavior.

7. **Wiring.** `internal/controller/<group>/<kind>_controller.go` with
   the RBAC markers and `Setup<Kind>Controller(mgr, api, opts)`. Register
   it in `cmd/main.go` next to its siblings, and add it to the group's
   `suite_test.go` envtest setup.

8. **Sample.** `config/samples/<group>_v1alpha1_<kind>.yaml` and an entry
   in `config/samples/kustomization.yaml`. It must apply cleanly:
   `kubectl apply -f <file> --dry-run=server` with CRDs installed.

9. **Docs row.** Add a row to `docs/resources.md`: kind, UpCloud
   counterpart, identity in status, produced Secrets, notes.

10. **CRD manifest registration.** Register the generated manifest in
    `config/crd/kustomization.yaml` (under the group's other entries):
    kustomize silently ignores files in `bases/` that the list does not
    name, so an unregistered kind installs no CRD. `make install`,
    `make deploy` and the release `install.yaml` all render from this
    list, and the miss only surfaces at apply time. `make lint` fails
    with a pointer to the file when a manifest is unregistered.

11. **Verify.** `mise run check`, then the group's envtest suite. The
    golden rules in `AGENTS.md` apply.

## Releasing

Releases are cut from tags; the `Release` workflow builds and pushes the
multi-arch image to `ghcr.io/polarsquad/uck`, signs it keylessly, builds
`dist/install.yaml`, and publishes the GitHub release in
`polarsquad/upcloud-operator`. Only releases cut after the rename use the
UCK image and Kubernetes object names; existing tags are not moved or
republished. Choose a new version for `TAG` below. Before the first UCK
release, confirm GHCR package ownership, Actions write access and public
read access for `polarsquad/uck`. The UpCloud branding/trademark question
in [#48](https://github.com/polarsquad/upcloud-operator/issues/48) still
requires maintainer confirmation.

```sh
git checkout main && git pull
TAG=vX.Y.Z # replace with the next release version
git tag -s "$TAG" -m "$TAG"
git push origin "$TAG"
```

Tags matching `*rc*`, `*alpha*` or `*beta*` create pre-releases.
Verify a release (the certificate identity embeds the tag ref):

```sh
docker manifest inspect "ghcr.io/polarsquad/uck:${TAG}"
cosign verify "ghcr.io/polarsquad/uck:${TAG}" \
  --certificate-identity "https://github.com/polarsquad/upcloud-operator/.github/workflows/release.yml@refs/tags/${TAG}" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Real-API e2e (admin setup)

The `e2e-upcloud` workflow is manual-only and gated behind the
`upcloud-e2e` GitHub environment. One-time admin steps (a repository
admin, with a UI):

1. Settings, Deploy environments: create an environment named
   `upcloud-e2e` with at least one required reviewer.
2. Settings, Secrets and variables, Actions: add the repository secret
   `UPCLOUD_TOKEN` (a scoped UpCloud API token).

Then trigger it from Actions, e2e (real UpCloud API), Run workflow.
One run provisions a real database for about 20 minutes.

### Teardown order

`AfterSuite` removes what the run created, even when a spec failed halfway
through. Every step runs whatever an earlier one did: a wait that times out
is recorded and the suite is failed once, at the end, with everything that
was recorded.

1. Collect the UpCloud identities of every managed CR still present.
2. Dump diagnostics while the kind cluster is still up.
3. Delete all CRs without blocking; the operator's finalizers perform the
   real UpCloud deletes.
4. Wait for the CRs to disappear (up to 30 minutes). Steps 3 and 4 are
   skipped when setup failed before the CRDs were installed; step 4 is
   also skipped when the spec already spent its own wait.
5. Sweep leftover networks, routers and floating IPs directly through the
   UpCloud API. This is a cost safety net, not part of what is tested, so
   it never hides an operator bug: each resource it has to remove is
   reported with the state of its CR (still present with its finalizers
   and Ready condition, or already gone) and the sweep's own delete error,
   and the suite fails at the end. Fix the operator's Delete, not the sweep.
6. Verify through the UpCloud API that everything is gone.
7. Delete the kind cluster, which takes the operator, CRDs and namespaces
   with it.

The suite cannot clean up when its own process is killed by a timeout
alarm, so the workflow ends with an `always()` step,
`go run ./test/e2e-upcloud/cleanup`, that keeps no state from the suite
and deletes every managed database, managed object storage, router and
network labelled `managed-by=uck`, plus detached floating IPs in the
sample zone. The step runs after a failed or timed-out suite; a manually
cancelled run skips it, so a cancel can still leak the run's resources
and needs a manual sweep. It retries for up to 15 minutes, because the
services delete asynchronously and a network cannot go until its router
has. The deadline is sized for that latency, and a delete still running
on the UpCloud side when it fires (a managed object storage delete has
taken over 20 minutes) is reported as in flight and the step passes: the
next dispatch re-sweeps anything that remains. The job fails only when
nothing is moving: no delete was accepted on the last pass, or a list
failed. Selecting by label alone is only safe because the UpCloud account
is dedicated to this suite; do not run the workflow against an account
that holds other operator-managed resources.

Runs are serialised by a `concurrency` group: every run shares the one
UpCloud account, so two at once could collide on network ranges and the
leak sweeps could delete the other run's resources. A dispatch made while
another run is active waits in the queue and is never cancelled, because
cancelling a run mid-teardown would leak its paid resources. GitHub keeps
only one pending run per group, so a third dispatch replaces the queued
second one.

### Real-API e2e timeouts

The suite's worst case is about 110.5m wall clock: BeforeSuite 2.5m,
controller-ready 3m, six 5m Ready waits, a 20m Managed Object Storage
Ready wait, a 15m database Ready wait, a 30m `waitForCRsGone` and a 5m
`waitForGone` in the spec, then AfterSuite's own 5m `waitForGone` after
its sweeps. AfterSuite skips `waitForCRsGone` when the spec already spent
that budget, so it is counted once.

Two alarms must stay ordered above that: `-ginkgo.timeout 120m` (Ginkgo's
own alarm, default 1h) below `-timeout 125m` (Go's alarm, default 10m).
If either fires inside teardown it kills the deletes and leaks the run's
paid resources into the next dispatch.

The budgets come from real runs. Managed Object Storage `setup-checkup`
took 13 to 15m to create (runs 35214495324 and 35271990226, the longest
14m42s), so the Ready budget is 20m. Deletion runs the same state machine
in reverse and exceeded 20m (run 35214495324: deletes issued at 11:35,
still present at 11:55, gone by about 12:05), so `waitForCRsGone` is 30m.
Tighten both once more runs record the durations.

## Known operational notes

- The credentials Secret is the only secret the operator reads. Scope
  the token to the minimum needed; rotate by replacing the Secret and
  restarting the Deployment (see the README).
- The `--steady-requeue` flag tunes how often Ready objects are
  re-checked for drift (default 5m).
