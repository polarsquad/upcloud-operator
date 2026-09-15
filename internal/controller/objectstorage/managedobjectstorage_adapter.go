package objectstorage

import (
	"context"
	"fmt"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"
	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud/request"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/polarsquad/upcloud-operator/api/common"
	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
	"github.com/polarsquad/upcloud-operator/internal/resolve"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi"
)

// ManagedObjectStorageAdapter maps ManagedObjectStorage onto the UpCloud
// Managed Object Storage API.
type ManagedObjectStorageAdapter struct {
	API    upcloudapi.ObjectStorageAPI
	Client client.Client
}

var _ reconciler.Adapter[*objectstoragev1alpha1.ManagedObjectStorage] = (*ManagedObjectStorageAdapter)(nil)

// lookup finds the service by status UUID, or by owner label across the list
// (the list endpoint has no label filter, so the filter is applied in Go).
func (a *ManagedObjectStorageAdapter) lookup(ctx context.Context, s *objectstoragev1alpha1.ManagedObjectStorage) (*upcloud.ManagedObjectStorage, error) {
	if s.Status.UUID != "" {
		svc, err := a.API.GetManagedObjectStorage(ctx, &request.GetManagedObjectStorageRequest{UUID: s.Status.UUID})
		if upcloudapi.IsNotFound(err) {
			return nil, nil
		}
		return svc, err
	}
	list, err := a.API.GetManagedObjectStorages(ctx, &request.GetManagedObjectStoragesRequest{})
	if err != nil {
		return nil, err
	}
	for i := range list {
		if upcloudapi.HasUID(list[i].Labels, s.UID) {
			return &list[i], nil
		}
	}
	return nil, nil
}

// desiredNetworks resolves the spec network attachments, turning networkRef
// entries into the referenced Network CR's UUID.
func (a *ManagedObjectStorageAdapter) desiredNetworks(ctx context.Context, s *objectstoragev1alpha1.ManagedObjectStorage) ([]upcloud.ManagedObjectStorageNetwork, error) {
	out := make([]upcloud.ManagedObjectStorageNetwork, 0, len(s.Spec.Networks))
	for _, att := range s.Spec.Networks {
		uuid, err := resolve.NetworkUUID(ctx, a.Client, s.Namespace, toCommonAtt(att))
		if err != nil {
			return nil, err
		}
		n := upcloud.ManagedObjectStorageNetwork{Name: att.Name, Type: att.Type, Family: att.Family}
		if uuid != "" {
			n.UUID = &uuid
		}
		out = append(out, n)
	}
	return out, nil
}

// toCommonAtt maps an MOS network attachment to the shared type so the
// network resolution helpers can be reused.
func toCommonAtt(a objectstoragev1alpha1.MOSNetworkAttachment) common.NetworkAttachment {
	return common.NetworkAttachment{Name: a.Name, Type: a.Type, Family: a.Family, UUID: a.UUID, NetworkRef: a.NetworkRef}
}

// Observe implements reconciler.Adapter.
func (a *ManagedObjectStorageAdapter) Observe(ctx context.Context, s *objectstoragev1alpha1.ManagedObjectStorage) (reconciler.Observation, error) {
	svc, err := a.lookup(ctx, s)
	if err != nil {
		return reconciler.Observation{}, fmt.Errorf("get managed object storage: %w", err)
	}
	if svc == nil {
		return reconciler.Observation{}, nil
	}
	s.Status.UUID = svc.UUID
	a.syncStatus(s, svc)

	networks, err := a.desiredNetworks(ctx, s)
	if err != nil {
		return reconciler.Observation{}, err
	}
	ready := stateReady(svc, s)
	return reconciler.Observation{
		Exists:   true,
		UpToDate: specMatches(svc, s, networks),
		Ready:    ready,
		Message:  "state " + string(svc.OperationalState),
	}, nil
}

// Create implements reconciler.Adapter.
func (a *ManagedObjectStorageAdapter) Create(ctx context.Context, s *objectstoragev1alpha1.ManagedObjectStorage) error {
	networks, err := a.desiredNetworks(ctx, s)
	if err != nil {
		return err
	}
	req := &request.CreateManagedObjectStorageRequest{
		Name:                  s.ExternalName(),
		Region:                s.Spec.Region,
		ConfiguredStatus:      upcloud.ManagedObjectStorageConfiguredStatus(s.DesiredConfiguredStatus()),
		Networks:              networks,
		TerminationProtection: s.Spec.TerminationProtection,
		Labels:                upcloudapi.DesiredLabels(s, s.Spec.Labels),
	}
	created, err := a.API.CreateManagedObjectStorage(ctx, req)
	if err != nil {
		return fmt.Errorf("create managed object storage: %w", err)
	}
	s.Status.UUID = created.UUID
	return nil
}

// Update implements reconciler.Adapter. Each drifting field is sent with a
// pointer in a single modify call.
func (a *ManagedObjectStorageAdapter) Update(ctx context.Context, s *objectstoragev1alpha1.ManagedObjectStorage) error {
	svc, err := a.lookup(ctx, s)
	if err != nil {
		return fmt.Errorf("get managed object storage: %w", err)
	}
	if svc == nil {
		return fmt.Errorf("update managed object storage: %w", reconciler.ErrPending)
	}

	networks, err := a.desiredNetworks(ctx, s)
	if err != nil {
		return err
	}

	modify := &request.ModifyManagedObjectStorageRequest{UUID: svc.UUID}
	if svc.Name != s.ExternalName() {
		name := s.ExternalName()
		modify.Name = &name
	}
	desired := upcloud.ManagedObjectStorageConfiguredStatus(s.DesiredConfiguredStatus())
	if svc.ConfiguredStatus != desired {
		modify.ConfiguredStatus = &desired
	}
	if svc.TerminationProtection != s.Spec.TerminationProtection {
		modify.TerminationProtection = &s.Spec.TerminationProtection
	}
	labels := upcloudapi.DesiredLabels(s, s.Spec.Labels)
	if !upcloudapi.LabelsEqual(svc.Labels, labels) {
		modify.Labels = &labels
	}
	if !mosNetworksEqual(svc.Networks, networks) {
		modify.Networks = &networks
	}

	if modify.Name == nil && modify.ConfiguredStatus == nil && modify.TerminationProtection == nil &&
		modify.Labels == nil && modify.Networks == nil {
		return nil
	}
	if _, err := a.API.ModifyManagedObjectStorage(ctx, modify); err != nil {
		return fmt.Errorf("modify managed object storage: %w", err)
	}
	return nil
}

// Delete implements reconciler.Adapter. A service with buckets is refused
// (409) and reported as pending with the API title; a 404 counts as deleted;
// a successful delete request is pending until the service is gone.
func (a *ManagedObjectStorageAdapter) Delete(ctx context.Context, s *objectstoragev1alpha1.ManagedObjectStorage) error {
	if s.Status.UUID == "" {
		return nil
	}
	svc, err := a.API.GetManagedObjectStorage(ctx, &request.GetManagedObjectStorageRequest{UUID: s.Status.UUID})
	if upcloudapi.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get managed object storage: %w", err)
	}
	if svc.OperationalState == upcloud.ManagedObjectStorageOperationalStateDeleteService ||
		svc.OperationalState == upcloud.ManagedObjectStorageOperationalStateDeleteDNS ||
		svc.OperationalState == upcloud.ManagedObjectStorageOperationalStateDeleteNetwork {
		return reconciler.ErrPending
	}
	err = a.API.DeleteManagedObjectStorage(ctx, &request.DeleteManagedObjectStorageRequest{UUID: s.Status.UUID, Force: false})
	switch {
	case err == nil:
		return reconciler.ErrPending
	case upcloudapi.IsNotFound(err):
		return nil
	case upcloudapi.IsConflict(err):
		// Buckets (or termination protection) block deletion. Surface the
		// API title so the user knows to delete the buckets first.
		return fmt.Errorf("%w: %s", reconciler.ErrPending, upcloudapi.TitleOf(err))
	default:
		return fmt.Errorf("delete managed object storage: %w", err)
	}
}

// stateReady reports running, or stopped when the spec asks for stopped.
func stateReady(svc *upcloud.ManagedObjectStorage, s *objectstoragev1alpha1.ManagedObjectStorage) bool {
	switch svc.OperationalState {
	case upcloud.ManagedObjectStorageOperationalStateRunning:
		return true
	case upcloud.ManagedObjectStorageOperationalStateStopped:
		return s.DesiredConfiguredStatus() == "stopped"
	default:
		return false
	}
}

func (a *ManagedObjectStorageAdapter) syncStatus(s *objectstoragev1alpha1.ManagedObjectStorage, svc *upcloud.ManagedObjectStorage) {
	s.Status.OperationalState = string(svc.OperationalState)
	endpoints := make([]objectstoragev1alpha1.Endpoint, 0, len(svc.Endpoints))
	for _, e := range svc.Endpoints {
		endpoints = append(endpoints, objectstoragev1alpha1.Endpoint{
			DomainName: e.DomainName,
			Type:       e.Type,
			IAMURL:     e.IAMURL,
			STSURL:     e.STSURL,
		})
	}
	s.Status.Endpoints = endpoints
}

// specMatches compares the mutable spec fields against the service.
func specMatches(svc *upcloud.ManagedObjectStorage, s *objectstoragev1alpha1.ManagedObjectStorage, networks []upcloud.ManagedObjectStorageNetwork) bool {
	if svc.Name != s.ExternalName() {
		return false
	}
	if svc.Region != s.Spec.Region {
		return false
	}
	if svc.TerminationProtection != s.Spec.TerminationProtection {
		return false
	}
	if svc.ConfiguredStatus != upcloud.ManagedObjectStorageConfiguredStatus(s.DesiredConfiguredStatus()) {
		return false
	}
	if !upcloudapi.LabelsEqual(svc.Labels, upcloudapi.DesiredLabels(s, s.Spec.Labels)) {
		return false
	}
	return mosNetworksEqual(svc.Networks, networks)
}

func mosNetworkKey(n upcloud.ManagedObjectStorageNetwork) string {
	uuid := ""
	if n.UUID != nil {
		uuid = *n.UUID
	}
	return n.Name + "|" + n.Type + "|" + n.Family + "|" + uuid
}

func mosNetworksEqual(a, b []upcloud.ManagedObjectStorageNetwork) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[string]bool, len(a))
	for _, n := range a {
		set[mosNetworkKey(n)] = true
	}
	for _, n := range b {
		if !set[mosNetworkKey(n)] {
			return false
		}
	}
	return true
}
