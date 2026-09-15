package objectstorage

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/polarsquad/upcloud-operator/api/common"
	objectstoragev1alpha1 "github.com/polarsquad/upcloud-operator/api/objectstorage/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
)

// Object names are prefixed with "mos-" so the single spec's objects do not
// collide with any other object in the shared envtest cluster.
const (
	envtestSvc  = "mos-svc"
	envtestPol  = "mos-policy"
	envtestUser = "mos-user"
	envtestKey  = "mos-key"
	envtestBkt  = "mos-bucket"
)

var _ = Describe("Object storage group end to end against the fake API", func() {
	It("creates a service, policy, user, access key and bucket, then deletes them in reverse", func(ctx SpecContext) {
		svc := &objectstoragev1alpha1.ManagedObjectStorage{
			ObjectMeta: metav1.ObjectMeta{Name: envtestSvc, Namespace: testNS},
			Spec:       objectstoragev1alpha1.ManagedObjectStorageSpec{Region: "fi-hel1"},
		}
		Expect(k8sClient.Create(ctx, svc)).To(Succeed())

		// The service reaches Ready.
		Eventually(func(g Gomega) {
			var got objectstoragev1alpha1.ManagedObjectStorage
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(svc), &got)).To(Succeed())
			g.Expect(reconciler.IsReady(&got)).To(BeTrue())
			g.Expect(got.Status.UUID).NotTo(BeEmpty())
			g.Expect(got.Status.Endpoints).NotTo(BeEmpty())
		}, "20s", "250ms").Should(Succeed())

		policy := &objectstoragev1alpha1.ObjectStoragePolicy{
			ObjectMeta: metav1.ObjectMeta{Name: envtestPol, Namespace: testNS},
			Spec: objectstoragev1alpha1.ObjectStoragePolicySpec{
				ServiceRef: common.LocalObjectReference{Name: envtestSvc},
				Name:       testROPolicy,
				Document:   `{"Version":"2012-10-17","Statement":[]}`,
			},
		}
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		user := &objectstoragev1alpha1.ObjectStorageUser{
			ObjectMeta: metav1.ObjectMeta{Name: envtestUser, Namespace: testNS},
			Spec: objectstoragev1alpha1.ObjectStorageUserSpec{
				ServiceRef: common.LocalObjectReference{Name: envtestSvc},
				Username:   "appuser",
				Policies:   []string{testROPolicy},
			},
		}
		Expect(k8sClient.Create(ctx, user)).To(Succeed())

		key := &objectstoragev1alpha1.ObjectStorageAccessKey{
			ObjectMeta: metav1.ObjectMeta{Name: envtestKey, Namespace: testNS},
			Spec: objectstoragev1alpha1.ObjectStorageAccessKeySpec{
				UserRef: common.LocalObjectReference{Name: envtestUser},
			},
		}
		Expect(k8sClient.Create(ctx, key)).To(Succeed())

		bucket := &objectstoragev1alpha1.ObjectStorageBucket{
			ObjectMeta: metav1.ObjectMeta{Name: envtestBkt, Namespace: testNS},
			Spec: objectstoragev1alpha1.ObjectStorageBucketSpec{
				ServiceRef: common.LocalObjectReference{Name: envtestSvc},
				Name:       "app-bucket",
			},
		}
		Expect(k8sClient.Create(ctx, bucket)).To(Succeed())

		// All children reach Ready; the access key's S3 Secret has four keys.
		Eventually(func(g Gomega) {
			var p objectstoragev1alpha1.ObjectStoragePolicy
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(policy), &p)).To(Succeed())
			g.Expect(reconciler.IsReady(&p)).To(BeTrue())

			var u objectstoragev1alpha1.ObjectStorageUser
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(user), &u)).To(Succeed())
			g.Expect(reconciler.IsReady(&u)).To(BeTrue())
			g.Expect(u.Status.AttachedPolicies).To(ConsistOf(testROPolicy))

			var k objectstoragev1alpha1.ObjectStorageAccessKey
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(key), &k)).To(Succeed())
			g.Expect(reconciler.IsReady(&k)).To(BeTrue())
			g.Expect(k.Status.AccessKeyID).NotTo(BeEmpty())

			var b objectstoragev1alpha1.ObjectStorageBucket
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bucket), &b)).To(Succeed())
			g.Expect(reconciler.IsReady(&b)).To(BeTrue())
			g.Expect(b.Status.Name).To(Equal("app-bucket"))
		}, "20s", "250ms").Should(Succeed())

		// The access key's S3 Secret holds exactly the four S3 keys.
		var s3Secret corev1.Secret
		secretKey := client.ObjectKey{Namespace: testNS, Name: envtestKey + "-s3"}
		Eventually(func(g Gomega) error {
			return k8sClient.Get(ctx, secretKey, &s3Secret)
		}, "20s", "250ms").Should(Succeed())
		for _, k := range []string{SecretKeyAccessKeyID, SecretKeySecretAccessKey, SecretKeyEndpointURL, SecretKeyRegion} {
			Expect(s3Secret.Data).To(HaveKey(k), "secret missing key %s", k)
		}

		// Delete in reverse: bucket, access key, user, policy, service.
		Expect(k8sClient.Delete(ctx, bucket)).To(Succeed())
		Expect(k8sClient.Delete(ctx, key)).To(Succeed())
		Expect(k8sClient.Delete(ctx, user)).To(Succeed())
		Expect(k8sClient.Delete(ctx, policy)).To(Succeed())
		Expect(k8sClient.Delete(ctx, svc)).To(Succeed())

		// Deleting the whole chain empties the fake.
		Eventually(func() int {
			total := len(fakeAPI.Services)
			for _, us := range fakeAPI.Users {
				total += len(us)
			}
			for _, ps := range fakeAPI.Policies {
				total += len(ps)
			}
			for _, bs := range fakeAPI.Buckets {
				total += len(bs)
			}
			return total
		}, "20s", "250ms").Should(BeZero())
	})
})
