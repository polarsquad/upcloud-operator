package database

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/polarsquad/upcloud-operator/api/common"
	databasev1alpha1 "github.com/polarsquad/upcloud-operator/api/database/v1alpha1"
	"github.com/polarsquad/upcloud-operator/internal/reconciler"
)

var _ = Describe("Database group end to end against the fake API", func() {
	It("creates a service, then a user and a logical database referencing it, then deletes all", func(ctx SpecContext) {
		md := &databasev1alpha1.ManagedDatabase{
			ObjectMeta: metav1.ObjectMeta{Name: "mdb", Namespace: "default"},
			Spec:       databasev1alpha1.ManagedDatabaseSpec{Type: "pg", Plan: "3x25", Zone: "fi-hel1"},
		}
		Expect(k8sClient.Create(ctx, md)).To(Succeed())

		// The service reaches Ready and writes its connection Secret.
		var connSecret corev1.Secret
		secretKey := client.ObjectKey{Namespace: "default", Name: "mdb-connection"}
		Eventually(func(g Gomega) {
			var got databasev1alpha1.ManagedDatabase
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(md), &got)).To(Succeed())
			g.Expect(reconciler.IsReady(&got)).To(BeTrue())
			g.Expect(got.Status.UUID).To(HavePrefix("mdb-"))
		}, "20s", "250ms").Should(Succeed())
		Eventually(func() error {
			return k8sClient.Get(ctx, secretKey, &connSecret)
		}, "20s", "250ms").Should(Succeed())

		user := &databasev1alpha1.ManagedDatabaseUser{
			ObjectMeta: metav1.ObjectMeta{Name: "appuser", Namespace: "default"},
			Spec: databasev1alpha1.ManagedDatabaseUserSpec{
				ServiceRef: common.LocalObjectReference{Name: "mdb"},
			},
		}
		Expect(k8sClient.Create(ctx, user)).To(Succeed())

		ldb := &databasev1alpha1.ManagedDatabaseLogicalDatabase{
			ObjectMeta: metav1.ObjectMeta{Name: "analytics", Namespace: "default"},
			Spec: databasev1alpha1.ManagedDatabaseLogicalDatabaseSpec{
				ServiceRef: common.LocalObjectReference{Name: "mdb"},
				Name:       "analytics",
			},
		}
		Expect(k8sClient.Create(ctx, ldb)).To(Succeed())

		// Both children reach Ready; the user's credentials Secret exists.
		var userSecret corev1.Secret
		userSecretKey := client.ObjectKey{Namespace: "default", Name: "appuser-credentials"}
		Eventually(func(g Gomega) {
			var u databasev1alpha1.ManagedDatabaseUser
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(user), &u)).To(Succeed())
			g.Expect(reconciler.IsReady(&u)).To(BeTrue())

			var l databasev1alpha1.ManagedDatabaseLogicalDatabase
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ldb), &l)).To(Succeed())
			g.Expect(reconciler.IsReady(&l)).To(BeTrue())
			g.Expect(l.Status.Name).To(Equal("analytics"))
		}, "20s", "250ms").Should(Succeed())
		Eventually(func() error {
			return k8sClient.Get(ctx, userSecretKey, &userSecret)
		}, "20s", "250ms").Should(Succeed())

		// Deleting the children and the service empties the fake.
		Expect(k8sClient.Delete(ctx, user)).To(Succeed())
		Expect(k8sClient.Delete(ctx, ldb)).To(Succeed())
		Expect(k8sClient.Delete(ctx, md)).To(Succeed())
		Eventually(func() int {
			total := len(fakeAPI.Databases)
			for _, dbs := range fakeAPI.LogicalDBs {
				total += len(dbs)
			}
			return total
		}, "20s", "250ms").Should(BeZero())
	})
})
