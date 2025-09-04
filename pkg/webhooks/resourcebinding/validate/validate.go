package validate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	admissionv1 "k8s.io/api/admission/v1"
	registrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"volcano.sh/volcano-global/pkg/webhooks/cache"
	"volcano.sh/volcano-global/pkg/webhooks/decoder"
	"volcano.sh/volcano-global/pkg/webhooks/router"
	"volcano.sh/volcano-global/pkg/webhooks/util"
	"volcano.sh/volcano-global/pkg/workload"
)

var Service = &router.AdmissionService{
	Path: "/resourcebindings/validate",
	Func: ResourceBindings,
	ValidatingConfig: &registrationv1.ValidatingWebhookConfiguration{
		Webhooks: []registrationv1.ValidatingWebhook{{
			Name: "validateresourcebindings.volcano.sh",
			Rules: []registrationv1.RuleWithOperations{
				{
					Operations: []registrationv1.OperationType{registrationv1.Create, registrationv1.Update},
					Rule: registrationv1.Rule{
						APIGroups:   []string{workv1alpha2.GroupVersion.Group},
						APIVersions: []string{workv1alpha2.GroupVersion.Version},
						Resources:   []string{workv1alpha2.ResourcePluralResourceBinding},
					},
				},
			},
		}},
	},
}

func ResourceBindings(ar admissionv1.AdmissionReview, snapshot *cache.DispatcherCacheSnapshot) *admissionv1.AdmissionResponse {
	var rb *workv1alpha2.ResourceBinding
	var oldRb *workv1alpha2.ResourceBinding
	var wl workload.Workload
	var err error
	rb, err = decoder.DecodeResourceBinding(ar.Request.Object, ar.Request.Resource)
	if err != nil {
		return util.Errored(http.StatusBadRequest, err)
	}
	if ar.Request.Operation == admissionv1.Create && len(rb.Spec.Clusters) == 0 {
		klog.V(4).Infof("ResourceBinding <%s/%s> is being created but not yet scheduled, skip it.", rb.Namespace, rb.Name)
		return util.Allowed("")
	}
	if ar.Request.Operation == admissionv1.Update {
		oldRb, err = decoder.DecodeResourceBinding(ar.Request.OldObject, ar.Request.Resource)
		if err != nil {
			return util.Errored(http.StatusBadRequest, err)
		}
		if !isResourceRequestChanged(oldRb, rb) {
			klog.V(4).Infof("ResourceBinding <%s/%s> resoruce request not changed, skip it.", rb.Namespace, rb.Name)
			return util.Allowed("")
		}
		if oldRb.Spec.Placement.ReplicaScheduling.ReplicaSchedulingType != rb.Spec.Placement.ReplicaScheduling.ReplicaSchedulingType {
			klog.V(4).Infof("ResourceBinding <%s/%s> replica scheduling type changed, skip it.", rb.Namespace, rb.Name)
			return util.Allowed("")
		}
	}
	rbResReq := cache.CalculateResourceRequest(rb)
	klog.V(4).Infof("New ResourceBinding <%s/%s> calculated resource request: %v", rb.Namespace, rb.Name, rbResReq)
	oldRbResReq := cache.CalculateResourceRequest(oldRb)
	klog.V(4).Infof("Old ResourceBinding <%s/%s> calculated resource request: %v", rb.Namespace, rb.Name, oldRbResReq)
	if rbResReq.IsEmpty() {
		klog.V(4).Infof("New ResourceBinding <%s/%s> resource request is empty, skip it.", rb.Namespace, rb.Name)
		return util.Allowed("")
	}
	wl, err = snapshot.GetResourceBindingWorkload(rb)
	if err != nil {
		klog.Errorf("ResourceBinding <%s/%s> not found workload, err: %v, skip it.", rb.Namespace, rb.Name, err)
		return util.Allowed("")
	}
	queue, found := snapshot.QueueInfos[wl.QueueName()]
	if !found {
		klog.V(4).Infof("ResourceBinding <%s/%s> not found queue, skip it.", rb.Namespace, rb.Name)
		return util.Allowed("")
	}
	attr := snapshot.QueueOpts[queue.UID]
	allocated := attr.Allocated.Clone()
	if !oldRbResReq.IsEmpty() {
		allocated.Sub(oldRbResReq)
	}
	futureUsed := allocated.Clone().Add(rbResReq)
	allocatable := futureUsed.LessEqualWithDimension(attr.RealCapability, rbResReq)
	if !allocatable {
		klog.V(4).Infof("Queue <%v>: realCapability <%v>, allocated <%v>; Candidate <%v/%v>: resource request <%v>",
			queue.Name, attr.RealCapability, allocated, rb.Namespace, rb.Name, rbResReq)
		err = fmt.Errorf("Queue <%s> is overused when considering ResourceBinding <%s/%s>", queue.Name, rb.Namespace, rb.Name)
		return util.Denied(err.Error())
	}
	klog.V(4).Infof("ResourceBinding <%s/%s> allocatable check pass!", rb.Namespace, rb.Name)
	return util.Allowed("")

}

func isResourceRequestChanged(oldRB, newRB *workv1alpha2.ResourceBinding) bool {
	if oldRB == nil || newRB == nil {
		return true
	}
	oldReq := corev1.ResourceList{}
	if oldRB.Spec.ReplicaRequirements != nil {
		oldReq = oldRB.Spec.ReplicaRequirements.ResourceRequest
	}
	newReq := corev1.ResourceList{}
	if newRB.Spec.ReplicaRequirements != nil {
		newReq = newRB.Spec.ReplicaRequirements.ResourceRequest
	}
	if !areResourceListsEqual(oldReq, newReq) {
		return true
	}
	oldScheduledReplicas := int32(0)
	for _, c := range oldRB.Spec.Clusters {
		oldScheduledReplicas += c.Replicas
	}
	newScheduledReplicas := int32(0)
	for _, c := range newRB.Spec.Clusters {
		newScheduledReplicas += c.Replicas
	}
	return oldScheduledReplicas != newScheduledReplicas
}

func areResourceListsEqual(a, b corev1.ResourceList) bool {
	if len(a) != len(b) {
		return false
	}
	for key, valA := range a {
		valB, ok := b[key]
		if !ok {
			return false
		}
		if valA.Cmp(valB) != 0 {
			return false
		}
	}
	return true
}

func placementChanged(
	placement policyv1alpha1.Placement,
	appliedPlacementStr string,
	schedulerObservingAffinityName string,
) bool {
	if appliedPlacementStr == "" {
		return true
	}

	appliedPlacement := policyv1alpha1.Placement{}
	err := json.Unmarshal([]byte(appliedPlacementStr), &appliedPlacement)
	if err != nil {
		klog.Errorf("Failed to unmarshal applied placement string: %v", err)
		return false
	}

	// first check: entire placement does not change
	if reflect.DeepEqual(placement, appliedPlacement) {
		return false
	}

	// second check: except for ClusterAffinities, the placement has changed
	if !reflect.DeepEqual(placement.ClusterAffinity, appliedPlacement.ClusterAffinity) ||
		!reflect.DeepEqual(placement.ClusterTolerations, appliedPlacement.ClusterTolerations) ||
		!reflect.DeepEqual(placement.SpreadConstraints, appliedPlacement.SpreadConstraints) ||
		!reflect.DeepEqual(placement.ReplicaScheduling, appliedPlacement.ReplicaScheduling) {
		return true
	}

	// third check: check weather ClusterAffinities has changed
	return clusterAffinitiesChanged(placement.ClusterAffinities, appliedPlacement.ClusterAffinities, schedulerObservingAffinityName)
}

func clusterAffinitiesChanged(
	clusterAffinities, appliedClusterAffinities []policyv1alpha1.ClusterAffinityTerm,
	schedulerObservingAffinityName string,
) bool {
	if schedulerObservingAffinityName == "" {
		return true
	}

	var clusterAffinityTerm, appliedClusterAffinityTerm *policyv1alpha1.ClusterAffinityTerm
	for index := range clusterAffinities {
		if clusterAffinities[index].AffinityName == schedulerObservingAffinityName {
			clusterAffinityTerm = &clusterAffinities[index]
			break
		}
	}
	for index := range appliedClusterAffinities {
		if appliedClusterAffinities[index].AffinityName == schedulerObservingAffinityName {
			appliedClusterAffinityTerm = &appliedClusterAffinities[index]
			break
		}
	}
	if clusterAffinityTerm == nil || appliedClusterAffinityTerm == nil {
		return true
	}
	if !reflect.DeepEqual(&clusterAffinityTerm, &appliedClusterAffinityTerm) {
		return true
	}
	return false
}
