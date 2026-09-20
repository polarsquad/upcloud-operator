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

## Known operational notes

- The credentials Secret is the only secret the operator reads. Scope
  the token to the minimum needed; rotate by replacing the Secret and
  restarting the Deployment (see the README).
- The `--steady-requeue` flag tunes how often Ready objects are
  re-checked for drift (default 5m).
