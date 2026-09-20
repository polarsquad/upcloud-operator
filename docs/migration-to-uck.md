# Migrating to UCK

UCK stands for **UpCloud Controllers for Kubernetes**, formerly
`upcloud-operator`. The name follows the convention used by ACK, ASO, and
KCC; see [the naming proposal](https://github.com/polarsquad/upcloud-operator/issues/48).

This guide applies to the first release containing the rename and later
releases. The rename PR does not publish an image or release. Wait for a
published UCK release before upgrading; existing release tags and images
are not republished under the new name.

## Changed identifiers

| Surface | Before | After |
|---|---|---|
| Project name | `upcloud-operator` | UCK (UpCloud Controllers for Kubernetes) |
| Kubebuilder `projectName` | `upcloud-operator` | `uck` |
| Release image | `ghcr.io/polarsquad/upcloud-operator:<tag>` | `ghcr.io/polarsquad/uck:<tag>` |
| Controller namespace | `upcloud-operator-system` | `uck-system` |
| Kustomize object-name prefix | `upcloud-operator-` | `uck-` |
| Deployment and ServiceAccount | `upcloud-operator-controller-manager` | `uck-controller-manager` |
| Metrics Service | `upcloud-operator-controller-manager-metrics-service` | `uck-controller-manager-metrics-service` |
| Kubernetes application label | `app.kubernetes.io/name=upcloud-operator` | `app.kubernetes.io/name=uck` |
| UpCloud ownership label | `managed-by=upcloud-operator` | `managed-by=uck` |
| UpCloud HTTP user agent | `upcloud-operator` | `uck` |
| Default smoke-test kind cluster | `upcloud-operator-test-e2e` | `uck-test-e2e` |
| Buildx builder | `upcloud-operator-builder` | `uck-builder` |

Role, ClusterRole, binding, and optional monitoring object names also use
`uck-`. Update external references to those objects, not just the
Deployment name.

## Unchanged identifiers

- GitHub repository and release downloads: `polarsquad/upcloud-operator`.
  The repository itself is not renamed by this change.
- Go module, imports, and Kubebuilder `repo` and resource `path` values:
  `github.com/polarsquad/upcloud-operator`. A module-path migration must be
  coordinated with a future repository rename; `github.com/polarsquad/uck`
  is not the module path for this release.
- The four `*.upcloud.polarsquad.com` CRD API groups, kind names, schemas,
  API versions, and finalizer strings.
- CR names, namespaces, UIDs, status identities, references, and generated
  workload Secrets. Do not recreate or move these objects.
- The adoption label `k8s-uid` and credentials Secret `upcloud-credentials`.
  Credential keys remain `UPCLOUD_TOKEN` or the pair `UPCLOUD_USERNAME`
  and `UPCLOUD_PASSWORD`.
- Leader-election ID `30a99488.upcloud.polarsquad.com`. Its Lease is
  namespaced, so the ID does not coordinate old and new installations in
  different namespaces.
- The release-signing workflow's certificate identity remains under
  `https://github.com/polarsquad/upcloud-operator/.github/workflows/release.yml`.
  Verify it against the tag used to build the image, as in the
  [README](../README.md#verify-the-image-signature).

## Upgrade an existing installation

The default manifests create a new controller installation, not an
in-place rename of the old Deployment. Kubernetes Deployment selectors
are immutable. Custom overlays that retain old object names while changing
selectors need their own replacement plan.

Only the controller moves to `uck-system`. It still watches CRs in all
namespaces. Keep CRs and their input/output Secrets in their current
namespaces, including any located in `upcloud-operator-system`.

1. **Prepare and record the current state.** Select a published UCK tag
   and verify the new image is pullable and its signature is valid. Save
   the old release tag, controller replica count, custom flags, resource
   settings, and manifest inventory for rollback. Securely back up CRs
   with their UIDs/status/finalizers and workload Secrets. Do not use a
   delete/recreate restore as a migration: a new CR UID breaks adoption,
   and some kinds cannot recover an identity from the API.

2. **Control automation.** Suspend or stage the GitOps/Helm automation
   managing the old installation so it cannot restart the old controller
   or prune CRDs, namespaces, CRs, or Secrets during the transition.
   Inventory custom metrics bindings, ServiceMonitors, network policies,
   and certificate names. Carry required custom settings into UCK.

3. **Stop the old controller and wait for its Pods to exit.** For the
   unmodified default installation:

   ```sh
   kubectl scale deployment upcloud-operator-controller-manager \
     -n upcloud-operator-system --replicas=0
   kubectl wait --for=delete pod \
     -n upcloud-operator-system \
     -l control-plane=controller-manager,app.kubernetes.io/name=upcloud-operator \
     --timeout=120s
   ```

   Stop if the wait fails. Check for custom or additional old manager
   Deployments too. Never run both controllers concurrently: each watches
   the same CRs, but their leader-election Leases are in different
   namespaces. Reconciliation pauses during the cutover; already-created
   cloud resources are not deleted by stopping the controller.

4. **Prepare credentials in the new namespace.** Create `uck-system` and
   provision `upcloud-credentials` there through your existing secret
   management process, using the same UpCloud account/workspace and
   permissions. Use one authentication method, not both. The install
   manifest does not create this Secret, and the Secret in the old
   namespace is not visible to the new Pod. Do not move workload Secrets.

5. **Install UCK.** With the old controller stopped and the new Secret
   ready, apply the selected release (or reconcile your updated GitOps
   configuration):

   ```sh
   TAG=vX.Y.Z # replace with a published UCK release tag
   kubectl apply -f "https://github.com/polarsquad/upcloud-operator/releases/download/${TAG}/install.yaml"
   kubectl rollout status deployment/uck-controller-manager \
     -n uck-system --timeout=180s
   ```

6. **Verify reconciliation, not just Pod readiness.** Confirm only UCK is
   running. Compare CR UIDs, status identities, finalizers, and workload
   Secrets with the recorded state. Check Ready conditions and controller
   logs for all managed kinds. Verify metrics collection after updating
   the Service name, namespace, selectors, certificate references, and
   role bindings. The readiness endpoint alone does not prove successful
   UpCloud API access.

7. **Retire only the old controller objects.** Keep the stopped installation
   during your rollback window. Afterwards, remove explicitly inventoried
   old Deployment, Service, ServiceAccount, controller roles/bindings, and
   leader-election Lease. Migrate external bindings before removing old
   metrics-reader or scaffolded admin/editor/viewer ClusterRoles. Inspect
   optional monitoring, network-policy, and certificate objects separately.
   Retain `upcloud-operator-system` if it contains any CRs or required
   Secrets. Resume automation with the UCK definitions and a reviewed
   prune inventory.

**Do not run `make undeploy`, `make uninstall`, or `kubectl delete -f` on
the complete old installer.** Those paths include CRDs and/or the old
namespace. Do not delete CRs or strip finalizers: default deletion policy
removes the real UpCloud resources.

## Cloud label reconciliation

New resources use `managed-by=uck`. Existing Networks, Routers, Gateways,
NetworkPeerings, ManagedDatabases, ManagedObjectStorages, and LoadBalancers
retain their status UUIDs and reconcile the old label through their
existing Modify paths. If status is empty, UID-based adoption still uses
`k8s-uid`, not the `managed-by` value. The rename does not require cloud
resource recreation.

These Modify requests can also resend other desired fields, and some
services must reach an accepted state before updates proceed. Treat the
cutover as a reconciliation change, not a guarantee of zero API activity
or zero service impact. User labels declared in `spec.labels` are retained;
out-of-band labels absent from that desired set may be removed, as before.

During the transition, inventory/audit tooling must recognize both
`managed-by=upcloud-operator` and `managed-by=uck`. Not every UpCloud resource
supports labels; use the CR's status identity for those resources. Do not
infer deletion from a missing management label.

## Rollback

Pause UCK automation, scale `uck-controller-manager` in `uck-system` to
zero, and wait for its Pods to terminate before restarting the old
controller with its previous replica count, configuration, and credentials.
Never restart the old controller while UCK is active. If the old controller
objects were removed, restore their installation manifests without deleting
shared CRDs or changing CR identities.

The preserved API groups, finalizers, status identities, and `k8s-uid`
allow the old controller to reconcile the same CRs. It may change cloud
labels back to `managed-by=upcloud-operator`; keep audits compatible with
both values. Recheck CR conditions and workload Secrets after rollback.

## Future repository rename

Renaming the GitHub repository, changing the Go module path, and updating
release/signature URLs require a separate coordinated change. Historical
image signatures retain the workflow identity used when they were signed.
No old GHCR image is moved or aliased by this PR. Maintainers must confirm
UpCloud branding/trademark approval and the new package's ownership and
access before publishing; this guide does not claim those checks are done.
