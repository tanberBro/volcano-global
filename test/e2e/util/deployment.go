package util

import (
	"context"
	"k8s.io/apimachinery/pkg/util/wait"
	"time"

	. "github.com/onsi/gomega"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateDeployment(ctx *TestContext, name string, rep int32, img string, req corev1.ResourceList) *appv1.Deployment {
	deploymentName := "deployment.k8s.io"
	d := &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ctx.Namespace,
		},
		Spec: appv1.DeploymentSpec{
			Replicas: &rep,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					deploymentName: name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{deploymentName: name},
					Annotations: map[string]string{"scheduling.volcano.sh/queue-name": ctx.Queues[0]},
				},
				Spec: corev1.PodSpec{
					SchedulerName: "volcano",
					RestartPolicy: corev1.RestartPolicyAlways,
					Containers: []corev1.Container{
						{
							Image:           img,
							Name:            name,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Resources: corev1.ResourceRequirements{
								Requests: req,
							},
						},
					},
				},
			},
		},
	}

	deployment, err := ctx.KubeClient.AppsV1().Deployments(ctx.Namespace).Create(context.Background(), d, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to create deployment %s", name)
	deployment.APIVersion = "apps/v1"
	deployment.Kind = "Deployment"
	return deployment
}

func WaitDeploymentReady(ctx *TestContext, name string) error {
	return wait.PollUntilContextTimeout(context.Background(), 100*time.Millisecond, FiveMinute, false, deploymentReady(ctx, name).WithContext())
}

func deploymentReady(ctx *TestContext, name string) wait.ConditionFunc {
	return func() (bool, error) {
		deployment, err := ctx.KubeClient.AppsV1().Deployments(ctx.Namespace).Get(context.Background(), name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to get deployment %s in namespace %s", name, ctx.Namespace)
		return *deployment.Spec.Replicas == deployment.Status.AvailableReplicas && deployment.Status.UnavailableReplicas == 0, nil
	}
}
