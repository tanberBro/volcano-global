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

package api

import (
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"k8s.io/apimachinery/pkg/types"
	volcanoapi "volcano.sh/volcano/pkg/scheduler/api"

	"volcano.sh/volcano-global/pkg/workload"
)

type ResourceBindingInfo struct {
	ResourceBinding *workv1alpha2.ResourceBinding
	ResourceUID     types.UID
	Queue           string
	ResReq          *volcanoapi.Resource
	Workload        workload.Workload
}

func (rbi *ResourceBindingInfo) DeepCopy() *ResourceBindingInfo {
	return &ResourceBindingInfo{
		ResourceBinding: rbi.ResourceBinding.DeepCopy(),
		ResourceUID:     rbi.ResourceUID,
		Queue:           rbi.Queue,
		ResReq:          rbi.ResReq.Clone(),
		Workload:        rbi.Workload,
	}
}
