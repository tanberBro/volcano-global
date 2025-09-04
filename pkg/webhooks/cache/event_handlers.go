/*
Copyright 2024 The Volcano Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cache

import (
	"context"
	"fmt"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog/v2"
	"volcano.sh/apis/pkg/apis/scheduling"
	"volcano.sh/apis/pkg/apis/scheduling/scheme"
	"volcano.sh/volcano-global/pkg/webhooks/api"
	schedulingapi "volcano.sh/volcano/pkg/scheduler/api"
	volcanoapi "volcano.sh/volcano/pkg/scheduler/api"

	"volcano.sh/volcano-global/pkg/utils"
	"volcano.sh/volcano-global/pkg/workload"
)

func (wc *WebhookCache) addQueue(obj interface{}) {
	queue := convertToQueue(obj)
	if queue == nil {
		return
	}

	// Convert the queue from v1beta1 to internal
	internalQueue := &scheduling.Queue{}
	if err := scheme.Scheme.Convert(queue, internalQueue, nil); err != nil {
		klog.Errorf("Failed to convert queue from %T to %T", queue, internalQueue)
		return
	}

	wc.mutex.Lock()
	defer wc.mutex.Unlock()

	wc.setQueue(internalQueue)
}

func (wc *WebhookCache) deleteQueue(obj interface{}) {
	queue := convertToQueue(obj)
	if queue == nil {
		return
	}
	wc.mutex.Lock()
	defer wc.mutex.Unlock()

	delete(wc.queues, queue.Name)
}

func (wc *WebhookCache) updateQueue(oldObj, newObj interface{}) {
	oldQueue := convertToQueue(oldObj)
	newQueue := convertToQueue(newObj)
	if oldQueue == nil || newQueue == nil {
		return
	}
	if oldQueue.ResourceVersion == newQueue.ResourceVersion {
		return
	}

	// Convert the queue from v1beta1 to internal
	internalQueue := &scheduling.Queue{}
	if err := scheme.Scheme.Convert(newQueue, internalQueue, nil); err != nil {
		klog.Errorf("Failed to convert queue from %T to %T", newQueue, internalQueue)
		return
	}

	wc.mutex.Lock()
	defer wc.mutex.Unlock()

	wc.setQueue(internalQueue)
}

func (wc *WebhookCache) setQueue(queue *scheduling.Queue) {
	wc.queues[queue.Name] = schedulingapi.NewQueueInfo(queue)
}

func (wc *WebhookCache) addResourceBinding(obj interface{}) {
	rb := convertToResourceBinding(obj)
	if rb == nil {
		return
	}

	wc.mutex.Lock()
	defer wc.mutex.Unlock()

	wc.setResourceBinding(rb)
}

func (wc *WebhookCache) deleteResourceBinding(obj interface{}) {
	rb := convertToResourceBinding(obj)
	if rb == nil {
		return
	}

	wc.mutex.Lock()
	defer wc.mutex.Unlock()

	if wc.resourceBindings[rb.Namespace] == nil {
		klog.Errorf("Failed to delete ResourceBinding <%s/%s>, the Resourcebinding's "+
			"Namespace is not in the cache.", rb.Namespace, rb.Name)
		return
	} else {
		delete(wc.resourceBindings[rb.Namespace], rb.Name)
		delete(wc.resourceBindingInfos[rb.Namespace], rb.Name)
	}
}

func (wc *WebhookCache) updateResourceBinding(oldObj, newObj interface{}) {
	oldRb := convertToResourceBinding(oldObj)
	newRb := convertToResourceBinding(newObj)
	if oldRb == nil || newRb == nil {
		return
	}
	if oldRb.ResourceVersion == newRb.ResourceVersion {
		return
	}

	wc.mutex.Lock()
	defer wc.mutex.Unlock()

	wc.setResourceBinding(newRb)
}

func (wc *WebhookCache) setResourceBinding(rb *workv1alpha2.ResourceBinding) {
	// Check if its workload, skip add to cache if not.
	wl, err := wc.getWorkload(rb.Spec.Resource)
	if err != nil {
		klog.Errorf("Failed to get resource from object reference <%s/%s>, err: %v",
			rb.Namespace, rb.Name, err)
		return
	}
	if wc.resourceBindings[rb.Namespace] == nil {
		wc.resourceBindings[rb.Namespace] = map[string]*workv1alpha2.ResourceBinding{
			rb.Name: rb,
		}
	} else {
		wc.resourceBindings[rb.Namespace][rb.Name] = rb
	}

	// Build the ResourceBindingInfo, the other elements will set when Snapshot.
	newResourceBindingInfo := &api.ResourceBindingInfo{
		ResourceBinding: rb,
		ResourceUID:     rb.Spec.Resource.UID,
		Queue:           wl.QueueName(),
		ResReq:          CalculateResourceRequest(rb),
		Workload:        wl,
	}

	if wc.resourceBindingInfos[rb.Namespace] == nil {
		wc.resourceBindingInfos[rb.Namespace] = map[string]*api.ResourceBindingInfo{
			rb.Name: newResourceBindingInfo,
		}
	} else {
		wc.resourceBindingInfos[rb.Namespace][rb.Name] = newResourceBindingInfo
	}
}

func (wc *WebhookCache) getWorkload(ref workv1alpha2.ObjectReference) (workload.Workload, error) {
	// Check if its workload, skip add to cache if not.
	isWorkload, newWorkloadFunc, err := workload.TryGetNewWorkloadFunc(ref)
	if err != nil {
		return nil, err
	}
	if !isWorkload {
		return nil, fmt.Errorf("ref is not workload")
	}

	resource, err := wc.getResourceFromObjectReference(ref)
	if err != nil {
		return nil, err
	}
	wl, err := newWorkloadFunc(resource)
	if err != nil {
		return nil, err
	}
	return wl, nil
}

func (wc *WebhookCache) addCluster(obj interface{}) {
	cluster := convertToCluster(obj)
	if cluster == nil {
		return
	}
	if ready, message := utils.CheckClusterReady(cluster); !ready {
		klog.V(5).Info(message)
		return
	}

	wc.mutex.Lock()
	defer wc.mutex.Unlock()

	wc.setCluster(cluster)
}

func (wc *WebhookCache) deleteCluster(obj interface{}) {
	cluster := convertToCluster(obj)
	if cluster == nil {
		return
	}

	wc.mutex.Lock()
	defer wc.mutex.Unlock()

	delete(wc.clusters, cluster.Name)
}

func (wc *WebhookCache) updateCluster(oldObj, newObj interface{}) {
	oldCluster := convertToCluster(oldObj)
	newCluster := convertToCluster(newObj)
	if oldCluster == nil || newCluster == nil {
		return
	}
	if oldCluster.ResourceVersion == newCluster.ResourceVersion {
		return
	}

	wc.mutex.Lock()
	defer wc.mutex.Unlock()

	wc.setCluster(newCluster)
}

func (wc *WebhookCache) setCluster(cluster *clusterv1alpha1.Cluster) {
	wc.clusters[cluster.Name] = cluster
}

func (wc *WebhookCache) getResourceFromObjectReference(ref workv1alpha2.ObjectReference) (*unstructured.Unstructured, error) {
	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		klog.Errorf("Failed to parse GroupVersion <%s>, err: %v", ref.APIVersion, err)
		return nil, err
	}
	mapping, err := wc.restMapper.RESTMapping(schema.GroupKind{
		Group: gv.Group,
		Kind:  ref.Kind,
	})

	if err != nil {
		klog.Errorf("Failed to get resource mapping from reference <%s/%s>, err: %v")
		return nil, err
	}

	// TODO: use informer instead.
	resource, err := wc.dynamicClient.Resource(mapping.Resource).Namespace(ref.Namespace).Get(context.Background(), ref.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get resource <%s/%s>, err: %v", ref.Namespace, ref.Name, err)
		return nil, err
	}
	return resource, nil
}

func CalculateResourceRequest(rb *workv1alpha2.ResourceBinding) *volcanoapi.Resource {
	resReq := volcanoapi.EmptyResource()
	if rb == nil || rb.Spec.ReplicaRequirements == nil || len(rb.Spec.ReplicaRequirements.ResourceRequest) == 0 || len(rb.Spec.Clusters) == 0 {
		return resReq
	}
	totalReplicas := int32(0)
	for _, cluster := range rb.Spec.Clusters {
		totalReplicas += cluster.Replicas
	}
	if totalReplicas == 0 {
		return resReq
	}
	resReq = volcanoapi.NewResource(rb.Spec.ReplicaRequirements.ResourceRequest).Multi(float64(totalReplicas))
	return resReq
}
