package util

import (
	"context"
	"github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Object interface {
	runtime.Object
	metav1.Object
}

func CreatePropagationPolicy(ctx *TestContext, obj Object, replicaSchedulingType v1alpha1.ReplicaSchedulingType) *v1alpha1.PropagationPolicy {
	pp := &v1alpha1.PropagationPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		},
		Spec: v1alpha1.PropagationSpec{
			ResourceSelectors: []v1alpha1.ResourceSelector{
				{
					APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
					Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
					Name:       obj.GetName()},
			},
			Placement: v1alpha1.Placement{
				ReplicaScheduling: &v1alpha1.ReplicaSchedulingStrategy{
					ReplicaSchedulingType: replicaSchedulingType,
				},
			},
		},
	}
	pp, err := ctx.KmClient.PolicyV1alpha1().PropagationPolicies(pp.Namespace).Create(context.Background(), pp, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to create propagationpolicy %s", pp.Name)
	return pp
}
