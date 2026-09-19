# Development

This guide covers the local loop, how to add a kind, and how to cut a
release.

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
make build-installer IMG=upcloud-operator:dev
kubectl apply -f dist/install.yaml
```

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
multi-arch image to GHCR, signs it keylessly, builds
`dist/install.yaml`, and publishes the GitHub release.

```sh
git checkout main && git pull
git tag -s v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

Tags matching `*rc*`, `*alpha*` or `*beta*` create pre-releases.
Verify a release (the certificate identity embeds the tag ref):

```sh
docker manifest inspect ghcr.io/polarsquad/upcloud-operator:v0.1.0
cosign verify ghcr.io/polarsquad/upcloud-operator:v0.1.0 \
  --certificate-identity "https://github.com/polarsquad/upcloud-operator/.github/workflows/release.yml@refs/tags/v0.1.0" \
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
4. Wait for the CRs to disappear (up to 20 minutes). Steps 3 and 4 are
   skipped when setup failed before the CRDs were installed.
5. Sweep leftover networks, routers and floating IPs directly through the
   UpCloud API. This is a cost safety net, not part of what is tested, so
   it never hides an operator bug: each resource it has to remove is
   reported with the state of its CR (still present with its finalizers
   and Ready condition, or already gone) and the sweep's own delete error,
   and the suite fails at the end. Fix the operator's Delete, not the sweep.
6. Verify through the UpCloud API that everything is gone.
7. Delete the kind cluster, which takes the operator, CRDs and namespaces
   with it.

The suite cannot clean up when its own process is killed (a timeout alarm
or a cancelled run). So the workflow ends with an `always()` step,
`go run ./test/e2e-upcloud/cleanup`, that keeps no state from the suite
and deletes every managed database, managed object storage, router and
network labelled `managed-by=upcloud-operator`, plus detached floating IPs
in the sample zone. It retries for up to 15 minutes, because the services
delete asynchronously and a network cannot go until its router has, and it
fails the job if anything remains. Selecting by label alone is only safe
because the UpCloud account is dedicated to this suite; do not run the
workflow against an account that holds other operator-managed resources.

## Known operational notes

- The credentials Secret is the only secret the operator reads. Scope
  the token to the minimum needed; rotate by replacing the Secret and
  restarting the Deployment (see the README).
- The `--steady-requeue` flag tunes how often Ready objects are
  re-checked for drift (default 5m).
