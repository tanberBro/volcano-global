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
	"fmt"
	"math"

	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"volcano.sh/volcano-global/pkg/webhooks/api"
	"volcano.sh/volcano-global/pkg/workload"
	schedulingapi "volcano.sh/volcano/pkg/scheduler/api"
	volcanoapi "volcano.sh/volcano/pkg/scheduler/api"
)

type DispatcherCacheSnapshot struct {
	// The map of the Queue Name to Queue info.
	DefaultQueue string
	QueueInfos   map[string]*schedulingapi.QueueInfo

	ResourceBindingInfos     map[string]map[types.UID]*api.ResourceBindingInfo
	ResourceBindingWorkloads map[types.UID]workload.Workload

	TotalResource *schedulingapi.Resource
	QueueOpts     map[volcanoapi.QueueID]*queueAttr
}

type queueAttr struct {
	QueueID    volcanoapi.QueueID
	Name       string
	share      float64
	Capability *volcanoapi.Resource
	// realCapacity represents the resource limit of the queue.
	RealCapability *volcanoapi.Resource
	// Allocated represents the resource request of all Allocated jobs in the queue.
	Allocated *volcanoapi.Resource
}

func (wc *WebhookCache) Snapshot() *DispatcherCacheSnapshot {
	wc.mutex.Lock()
	defer wc.mutex.Unlock()

	snapshot := &DispatcherCacheSnapshot{
		DefaultQueue:             wc.defaultQueue,
		QueueInfos:               make(map[string]*schedulingapi.QueueInfo, len(wc.queues)),
		ResourceBindingInfos:     make(map[string]map[types.UID]*api.ResourceBindingInfo),
		TotalResource:            schedulingapi.EmptyResource(),
		QueueOpts:                make(map[volcanoapi.QueueID]*queueAttr),
		ResourceBindingWorkloads: make(map[types.UID]workload.Workload),
	}

	for _, queue := range wc.queues {
		snapshot.QueueInfos[queue.Name] = queue.Clone()
	}

	for _, cluster := range wc.clusters {
		snapshot.TotalResource.Add(schedulingapi.NewResource(cluster.Status.ResourceSummary.Allocatable))
	}

	// Collect the ResourceBindingInfos.
	// First, we should update the elements of the ResourceBindingInfo,
	// because we only set some elements of them when creating the ResourceBindingInfo.
	for _, resourceBindingInfoMap := range wc.resourceBindingInfos {
		for _, rbi := range resourceBindingInfoMap {
			// Get the priority of the workload.

			// On the end, we need to copy it.
			if snapshot.ResourceBindingInfos[rbi.Queue] == nil {
				snapshot.ResourceBindingInfos[rbi.Queue] = make(map[types.UID]*api.ResourceBindingInfo)
			}
			snapshot.ResourceBindingInfos[rbi.Queue][rbi.ResourceBinding.UID] = rbi.DeepCopy()
			snapshot.ResourceBindingWorkloads[rbi.ResourceBinding.UID] = rbi.Workload
		}
	}

	snapshot.buildQueueAttrs()
	return snapshot
}

func (dcs *DispatcherCacheSnapshot) buildQueueAttrs() {
	for _, queue := range dcs.QueueInfos {
		var attr *queueAttr
		var found bool
		if attr, found = dcs.QueueOpts[queue.UID]; !found {
			attr = &queueAttr{
				QueueID:   queue.UID,
				Name:      queue.Name,
				Allocated: volcanoapi.EmptyResource(),
			}
			if len(queue.Queue.Spec.Capability) != 0 {
				attr.Capability = volcanoapi.NewResource(queue.Queue.Spec.Capability)
				if attr.Capability.MilliCPU <= 0 {
					attr.Capability.MilliCPU = math.MaxFloat64
				}
				if attr.Capability.Memory <= 0 {
					attr.Capability.Memory = math.MaxFloat64
				}
			}
			realCapability := dcs.TotalResource.Clone()
			if attr.Capability == nil {
				attr.RealCapability = realCapability
			} else {
				realCapability.MinDimensionResource(attr.Capability, volcanoapi.Infinity)
				attr.RealCapability = realCapability
			}
			dcs.QueueOpts[queue.UID] = attr
		}
		for _, rbi := range dcs.ResourceBindingInfos[queue.Name] {
			attr.Allocated = attr.Allocated.Add(rbi.ResReq)
		}
	}
	for _, attr := range dcs.QueueOpts {
		//dcs.updateShare(attr)
		klog.V(4).Infof("The attributes of queue <%s> in capacity: RealCapability <%v>, Allocated <%v>, share <%0.2f>.",
			attr.Name, attr.RealCapability, attr.Allocated, attr.share)
	}
}

// GetResourceBindingInfoQueue Get the workload's queue Name, it may be empty when DefaultQueue is empty.
func (dcs *DispatcherCacheSnapshot) GetResourceBindingInfoQueue(rbi *api.ResourceBindingInfo) string {
	name := rbi.Queue
	if name == "" {
		name = dcs.DefaultQueue
		resource := rbi.ResourceBinding.Spec.Resource
		klog.V(3).Infof("Resource %s <%s/%s> didnt set the Queue Name annotation, join the default queue <%s>.",
			resource.Kind, resource.Namespace, resource.Name, dcs.DefaultQueue)
	}
	return name
}

func (dcs *DispatcherCacheSnapshot) GetResourceBindingWorkload(rb *workv1alpha2.ResourceBinding) (workload.Workload, error) {
	wl, ok := dcs.ResourceBindingWorkloads[rb.UID]
	if ok {
		return wl, nil
	}
	return nil, fmt.Errorf("not found workload")

}
