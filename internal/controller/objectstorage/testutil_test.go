package objectstorage

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/UpCloudLtd/upcloud-go-api/v8/upcloud"

	"github.com/polarsquad/upcloud-operator/api/common"
	networkv1alpha1 "github.com/polarsquad/upcloud-operator/api/network/v1alpha1"
	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/upcloudapi/fake"
)

// Shared test constants. Kept in one place so the goconst linter (which
// counts occurrences per package) does not flag repeated literals.
const (
	testNS        = "default"
	parentName    = "svc"
	parentUUID    = "moss-0001"
	regionFinland = "fi-hel1"
	readyReason   = "Available"
	testUserName  = "alice"
	testUserUID   = "user-uid"
	testROPolicy  = "read-only"
)

// newMOSClient returns a fake client with the corev1, network and
// objectstorage schemes, for adapter unit tests.
func newMOSClient(t *testing.T) client.Client {
	g := NewWithT(t)
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())
	g.Expect(objectstoragev1alpha1.AddToScheme(s)).To(Succeed())
	g.Expect(networkv1alpha1.AddToScheme(s)).To(Succeed())
	return fakeclient.NewClientBuilder().WithScheme(s).Build()
}

// readyService creates a Ready ManagedObjectStorage CR named parentName and
// the matching fake service, for child adapter tests.
func readyService(g *GomegaWithT, c client.Client, api *fake.ObjectStorageAPI) {
	svc := &objectstoragev1alpha1.ManagedObjectStorage{
		ObjectMeta: metav1.ObjectMeta{Name: parentName, Namespace: testNS, UID: types.UID(parentName), Generation: 1},
		Spec:       objectstoragev1alpha1.ManagedObjectStorageSpec{Region: regionFinland},
		Status: objectstoragev1alpha1.ManagedObjectStorageStatus{
			UUID:             parentUUID,
			OperationalState: "running",
			Endpoints:        []objectstoragev1alpha1.Endpoint{{DomainName: parentUUID + ".upcloudobjects.com", Type: EndpointTypePublic}},
			Conditions:       []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue, Reason: readyReason, Message: "ok", ObservedGeneration: 1}},
		},
	}
	g.Expect(c.Create(context.Background(), svc)).To(Succeed())
	api.Services[parentUUID] = &upcloud.ManagedObjectStorage{
		UUID:             parentUUID,
		Name:             parentName,
		Region:           regionFinland,
		OperationalState: upcloud.ManagedObjectStorageOperationalStateRunning,
		ConfiguredStatus: upcloud.ManagedObjectStorageConfiguredStatusStarted,
		Endpoints:        []upcloud.ManagedObjectStorageEndpoint{{DomainName: parentUUID + ".upcloudobjects.com", Type: EndpointTypePublic}},
	}
}

// readyUser creates a Ready ObjectStorageUser named testUserName and the
// matching fake user, for access key adapter tests.
func readyUser(g *GomegaWithT, c client.Client, api *fake.ObjectStorageAPI) {
	u := &objectstoragev1alpha1.ObjectStorageUser{
		ObjectMeta: metav1.ObjectMeta{Name: testUserName, Namespace: testNS, UID: types.UID(testUserUID), Generation: 1},
		Spec:       objectstoragev1alpha1.ObjectStorageUserSpec{ServiceRef: common.LocalObjectReference{Name: parentName}},
		Status: objectstoragev1alpha1.ObjectStorageUserStatus{
			ServiceUUID: parentUUID,
			Username:    testUserName,
			ARN:         "arn:upcloud:objectstorage:" + regionFinland + "::user/" + testUserName,
			Conditions:  []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue, Reason: readyReason, Message: "ok", ObservedGeneration: 1}},
		},
	}
	g.Expect(c.Create(context.Background(), u)).To(Succeed())
	api.Users[parentUUID] = append(api.Users[parentUUID], upcloud.ManagedObjectStorageUser{
		Username: testUserName,
		ARN:      "arn:upcloud:objectstorage:" + regionFinland + "::user/" + testUserName,
	})
}
