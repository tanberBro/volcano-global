package util

import (
	"context"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	schedv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"os"
	"path/filepath"
	"time"
	schedulingv1beta1 "volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	vcclient "volcano.sh/apis/pkg/client/clientset/versioned"
	"volcano.sh/volcano/pkg/controllers/job/helpers"
)

var (
	FiveMinute = 5 * time.Minute

	KubeClient *kubernetes.Clientset
	VcClient   *vcclient.Clientset
	KmClient   *karmadaclientset.Clientset

	DefaultNginxImage = "nginx"
)

type TestContext struct {
	KubeClient *kubernetes.Clientset
	VcClient   *vcclient.Clientset
	KmClient   *karmadaclientset.Clientset

	Namespace        string
	Queues           []string
	CapacityResource map[string]v1.ResourceList
	PriorityClasses  map[string]int32
}

type Options struct {
	Namespace        string
	Queues           []string
	CapacityResource map[string]v1.ResourceList
	PriorityClasses  map[string]int32
}

func InitTestContext(o Options) *TestContext {
	By("Initializing test context")

	if o.Namespace == "" {
		o.Namespace = helpers.GenRandomStr(8)
	}
	ctx := &TestContext{
		Namespace:        o.Namespace,
		Queues:           o.Queues,
		CapacityResource: o.CapacityResource,
		PriorityClasses:  o.PriorityClasses,
		KubeClient:       KubeClient,
		VcClient:         VcClient,
		KmClient:         KmClient,
	}

	_, err := ctx.KubeClient.CoreV1().Namespaces().Create(context.TODO(),
		&v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ctx.Namespace,
			},
		},
		metav1.CreateOptions{},
	)
	Expect(err).NotTo(HaveOccurred(), "failed to create namespace")

	CreateQueues(ctx)
	CreatePriorityClasses(ctx)

	return ctx
}

func CleanupTestContext(ctx *TestContext) {
	By("Cleaning up test context")

	foreground := metav1.DeletePropagationForeground
	err := ctx.KubeClient.CoreV1().Namespaces().Delete(context.TODO(), ctx.Namespace, metav1.DeleteOptions{
		PropagationPolicy: &foreground,
	})
	Expect(err).NotTo(HaveOccurred(), "failed to delete namespace")

	DeleteQueues(ctx)
	DeletePriorityClasses(ctx)

	// Wait for namespace deleted.
	err = wait.PollUntilContextTimeout(context.Background(), 100*time.Millisecond, FiveMinute, false, NamespaceNotExist(ctx).WithContext())
	Expect(err).NotTo(HaveOccurred(), "failed to wait for namespace deleted")
}

func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func MasterURL() string {
	if m := os.Getenv("MASTER"); m != "" {
		return m
	}
	return ""
}

func KubeconfigPath(home string) string {
	if m := os.Getenv("KUBECONFIG"); m != "" {
		return m
	}
	return filepath.Join(home, ".kube", "config") // default kubeconfig path is $HOME/.kube/config
}

// CreateQueues create Queues specified in the test context
func CreateQueues(ctx *TestContext) {
	By("Creating Queues")

	for _, queue := range ctx.Queues {
		CreateQueue(ctx, queue, ctx.CapacityResource[queue])
	}

	// wait for all queues state open
	time.Sleep(3 * time.Second)
}

// CreateQueue creates Queue with the specified name
func CreateQueue(ctx *TestContext, q string, capacity v1.ResourceList) {
	_, err := ctx.VcClient.SchedulingV1beta1().Queues().Get(context.TODO(), q, metav1.GetOptions{})
	if err != nil {
		_, err := ctx.VcClient.SchedulingV1beta1().Queues().Create(context.TODO(), &schedulingv1beta1.Queue{
			ObjectMeta: metav1.ObjectMeta{
				Name: q,
			},
			Spec: schedulingv1beta1.QueueSpec{
				Weight:     1,
				Capability: capacity,
			},
		}, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to create queue %s", q)
	}
}

// DeleteQueues deletes Queues specified in the test context
func DeleteQueues(ctx *TestContext) {
	for _, q := range ctx.Queues {
		DeleteQueue(ctx, q)
	}
}

// DeleteQueue deletes Queue with the specified name
func DeleteQueue(ctx *TestContext, q string) {
	foreground := metav1.DeletePropagationForeground
	var queue *schedulingv1beta1.Queue
	var err error
	retryErr := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		queue, err = ctx.VcClient.SchedulingV1beta1().Queues().Get(context.TODO(), q, metav1.GetOptions{})
		if err != nil {
			Expect(err).NotTo(HaveOccurred(), "failed to get queue %s", q)
			return err
		}
		queue.Status.State = schedulingv1beta1.QueueStateClosed
		if _, err = ctx.VcClient.SchedulingV1beta1().Queues().UpdateStatus(context.TODO(), queue, metav1.UpdateOptions{}); err != nil {
			return err
		}
		return nil
	})
	Expect(retryErr).NotTo(HaveOccurred(), "failed to update status of queue %s", q)
	err = wait.PollUntilContextTimeout(context.Background(), 100*time.Millisecond, FiveMinute, false, QueueClosed(ctx, q).WithContext())
	Expect(err).NotTo(HaveOccurred(), "failed to wait queue %s closed", q)

	err = ctx.VcClient.SchedulingV1beta1().Queues().Delete(context.TODO(), q,
		metav1.DeleteOptions{
			PropagationPolicy: &foreground,
		})
	Expect(err).NotTo(HaveOccurred(), "failed to delete queue %s", q)
}

func CreatePriorityClasses(cxt *TestContext) {
	for name, value := range cxt.PriorityClasses {
		_, err := cxt.KubeClient.SchedulingV1().PriorityClasses().Create(context.TODO(),
			&schedv1.PriorityClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Value:         value,
				GlobalDefault: false,
			},
			metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to create priority class: %s", name)
	}
}

func DeletePriorityClasses(cxt *TestContext) {
	for name := range cxt.PriorityClasses {
		err := cxt.KubeClient.SchedulingV1().PriorityClasses().Delete(context.TODO(), name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	}
}

func NamespaceNotExist(ctx *TestContext) wait.ConditionFunc {
	return NamespaceNotExistWithName(ctx, ctx.Namespace)
}

func NamespaceNotExistWithName(ctx *TestContext, name string) wait.ConditionFunc {
	return func() (bool, error) {
		_, err := ctx.KubeClient.CoreV1().Namespaces().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil && errors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	}
}

// QueueClosed returns whether the Queue is closed
func QueueClosed(ctx *TestContext, name string) wait.ConditionFunc {
	return func() (bool, error) {
		queue, err := ctx.VcClient.SchedulingV1beta1().Queues().Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		if queue.Status.State != schedulingv1beta1.QueueStateClosed {
			return false, nil
		}

		return true, nil
	}
}
