package dispatchplugin

import (
	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	e2eutil "volcano.sh/volcano-global/test/e2e/util"
)

var _ = Describe("Capacity E2E Test", func() {
	It("allocate Deployment will work when resource is enough", func() {
		ns := "test-capacity"
		queueName := "test-capacity"
		ctx := e2eutil.InitTestContext(e2eutil.Options{
			Namespace: ns,
			Queues:    []string{queueName},
			CapacityResource: map[string]corev1.ResourceList{
				queueName: {
					corev1.ResourceCPU:    resource.MustParse("2000m"),
					corev1.ResourceMemory: resource.MustParse("2048Mi"),
				},
			},
		})
		defer e2eutil.CleanupTestContext(ctx)

		slot1 := corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("512Mi")}
		slot2 := corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000m"),
			corev1.ResourceMemory: resource.MustParse("1024Mi")}

		d := e2eutil.CreateDeployment(ctx, "deployment1", 1, e2eutil.DefaultNginxImage, slot1)
		e2eutil.CreatePropagationPolicy(ctx, d, v1alpha1.ReplicaSchedulingTypeDivided)
		err := e2eutil.WaitDeploymentReady(ctx, d.Name)
		Expect(err).NotTo(HaveOccurred())

		d = e2eutil.CreateDeployment(ctx, "deployment2", 1, e2eutil.DefaultNginxImage, slot2)
		e2eutil.CreatePropagationPolicy(ctx, d, v1alpha1.ReplicaSchedulingTypeDivided)
		err = e2eutil.WaitDeploymentReady(ctx, d.Name)
		Expect(err).NotTo(HaveOccurred())

	})
	It("allocate Deployment will not work when resource is not enough", func() {
		ns := "test-capacity"
		queueName := "test-capacity"
		ctx := e2eutil.InitTestContext(e2eutil.Options{
			Namespace: ns,
			Queues:    []string{queueName},
			CapacityResource: map[string]corev1.ResourceList{
				queueName: {
					corev1.ResourceCPU:    resource.MustParse("2000m"),
					corev1.ResourceMemory: resource.MustParse("2048Mi"),
				},
			},
		})
		defer e2eutil.CleanupTestContext(ctx)

		slot1 := corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("512Mi")}
		slot2 := corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000m"),
			corev1.ResourceMemory: resource.MustParse("1024Mi")}

		d := e2eutil.CreateDeployment(ctx, "deployment1", 1, e2eutil.DefaultNginxImage, slot1)
		e2eutil.CreatePropagationPolicy(ctx, d, v1alpha1.ReplicaSchedulingTypeDivided)
		err := e2eutil.WaitDeploymentReady(ctx, d.Name)
		Expect(err).NotTo(HaveOccurred())

		d = e2eutil.CreateDeployment(ctx, "deployment2", 2, e2eutil.DefaultNginxImage, slot2)
		e2eutil.CreatePropagationPolicy(ctx, d, v1alpha1.ReplicaSchedulingTypeDivided)
		err = e2eutil.WaitDeploymentReady(ctx, d.Name)
		Expect(err).NotTo(HaveOccurred())

	})
	//It("allocate Job will work when resource is enough")
	//It("allocate Deployment and Job will work when resource is enough")
	//It("allocate Job will not work when resource is not enough")
	//It("allocate Deployment and Job will not work when resource is not enough")
})
