env:

|      context      |          describe          |
| :---------------: | :------------------------: |
| karmada-apiserver | karmada controller cluster |
|    kind-tanber    |       member cluster       |
|   kind-member1    |       member cluster       |


1. test capacity in test(5C5Gi) queue.
+ create queue test
```bash
kubectl --context karmada-apiserver apply -f- <<EOF
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: test
spec:
  capability:
    cpu: "5"
    memory: 5Gi
  reclaimable: true
  weight: 1
EOF
```
+ test deployment nginx with `replicaSchedulingType: Divided, total resource request: 2C2Gi*2` in queue, it can scheduled.
```bash
kubectl --context karmada-apiserver apply -f- <<EOF 
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
      annotations:
        scheduling.volcano.sh/queue-name: test
    spec:
      schedulerName: volcano
      containers:
        - image: nginx
          name: nginx
          imagePullPolicy: IfNotPresent
          resources:
            requests:
              cpu: "2"
              memory: "2Gi"
            limits:
              cpu: "2"
              memory: "2Gi"
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    replicaScheduling:
      replicaSchedulingType: Divided
EOF
```
check rb and pod status
```bash
kubectl --context karmada-apiserver get rb
NAME               SCHEDULED   FULLYAPPLIED   AGE
nginx-deployment   True        True           18s
kubectl --context kind-tanber get pod     
NAME                     READY   STATUS    RESTARTS   AGE
nginx-5cb647754f-866rt   1/1     Running   0          27s
kubectl --context kind-member1  get pod
NAME                     READY   STATUS    RESTARTS   AGE
nginx-5cb647754f-j565r   1/1     Running   0          35s
```
+ test deployment update replicaSchedulingType to Duplicated, because total resource request will use 2C2Gi\*2\*2, greater than queue capacity, so it can't scheduled.
```bash
kubectl --context karmada-apiserver apply -f- <<EOF
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: nginx-propagation
spec:
  resourceSelectors:
    - apiVersion: apps/v1
      kind: Deployment
      name: nginx
  placement:
    replicaScheduling:
      replicaSchedulingType: Duplicated
EOF
```
check rb and pod status
```bash
kubectl --context kind-tanber get pod                             
NAME                     READY   STATUS    RESTARTS   AGE
nginx-5cb647754f-866rt   1/1     Running   0          13m
kubectl --context kind-member1  get pod
NAME                     READY   STATUS    RESTARTS   AGE
nginx-5cb647754f-j565r   1/1     Running   0          13m
kubectl --context karmada-apiserver get rb nginx-deployment       
NAME               SCHEDULED   FULLYAPPLIED   AGE
nginx-deployment   False       True           13m
kubectl --context karmada-apiserver get rb nginx-deployment -oyaml
apiVersion: work.karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  annotations:
    policy.karmada.io/applied-placement: '{"replicaScheduling":{"replicaSchedulingType":"Divided","replicaDivisionPreference":"Weighted"}}'
    propagationpolicy.karmada.io/name: nginx-propagation
    propagationpolicy.karmada.io/namespace: default
  creationTimestamp: "2025-09-04T01:12:03Z"
  finalizers:
  - karmada.io/binding-controller
  generation: 3
  labels:
    propagationpolicy.karmada.io/permanent-id: 306ba637-8ebc-420a-bdaf-d8e8cab746bc
    resourcebinding.karmada.io/permanent-id: 6ba998cc-bc28-46b9-96bb-18cc187440c5
  name: nginx-deployment
  namespace: default
  ownerReferences:
  - apiVersion: apps/v1
    blockOwnerDeletion: true
    controller: true
    kind: Deployment
    name: nginx
    uid: 3e7e5f0d-6e0e-4e24-9d73-4c8f1277c58c
  resourceVersion: "2652741"
  uid: c50b0a41-e957-40b1-939c-32f6bbeec584
spec:
  clusters:
  - name: kind-tanber
    replicas: 1
  - name: kind-member1
    replicas: 1
  conflictResolution: Abort
  placement:
    replicaScheduling:
      replicaDivisionPreference: Weighted
      replicaSchedulingType: Duplicated
  replicaRequirements:
    resourceRequest:
      cpu: "2"
      memory: 2Gi
  replicas: 2
  resource:
    apiVersion: apps/v1
    kind: Deployment
    name: nginx
    namespace: default
    resourceVersion: "2651390"
    uid: 3e7e5f0d-6e0e-4e24-9d73-4c8f1277c58c
  schedulerName: default-scheduler
status:
  aggregatedStatus:
  - applied: true
    clusterName: kind-member1
    health: Healthy
    status:
      availableReplicas: 1
      generation: 1
      observedGeneration: 1
      readyReplicas: 1
      replicas: 1
      resourceTemplateGeneration: 2
      updatedReplicas: 1
  - applied: true
    clusterName: kind-tanber
    health: Healthy
    status:
      availableReplicas: 1
      generation: 1
      observedGeneration: 1
      readyReplicas: 1
      replicas: 1
      resourceTemplateGeneration: 2
      updatedReplicas: 1
  conditions:
  - lastTransitionTime: "2025-09-04T01:25:10Z"
    message: 'admission webhook "validateresourcebindings.volcano.sh" denied the request:
      Queue <test> is overused when considering ResourceBinding <default/nginx-deployment>'
    reason: SchedulerError
    status: "False"
    type: Scheduled
  - lastTransitionTime: "2025-09-04T01:12:04Z"
    message: All works have been successfully applied
    reason: FullyAppliedSuccess
    status: "True"
    type: FullyApplied
  lastScheduledTime: "2025-09-04T01:12:03Z"
  schedulerObservedGeneration: 2

```
+ test deployment update total resource request to 1C1Gi\*2\*2, it can scheduled.
```bash
kubectl --context karmada-apiserver apply -f- <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
      annotations:
        scheduling.volcano.sh/queue-name: test
    spec:
      schedulerName: volcano
      containers:
        - image: nginx
          name: nginx
          imagePullPolicy: IfNotPresent
          resources:
            requests:
              cpu: "1"
              memory: "1Gi"
            limits:
              cpu: "1"
              memory: "1Gi"
EOF
```
check rb and pod status
```bash
kubectl --context karmada-apiserver get rb nginx-deployment 
NAME               SCHEDULED   FULLYAPPLIED   AGE
nginx-deployment   True        True           20m
kubectl --context kind-tanber get pod                       
NAME                     READY   STATUS    RESTARTS   AGE
nginx-78467bb9fc-7kc46   1/1     Running   0          16s
nginx-78467bb9fc-jlp24   1/1     Running   0          16s
kubectl --context kind-member1  get pod
NAME                     READY   STATUS    RESTARTS   AGE
nginx-78467bb9fc-xnpst   1/1     Running   0          20s
nginx-78467bb9fc-xr824   1/1     Running   0          20s
```
+ test vcjob mindspore-1-cpu with `replicaSchedulingType: Duplicated, total resource request: 0.5C0.5Gi*2*2` in queue, greater than queue allocatable( nginx used `4C4Gi`), so it can't scheduled.
```bash
kubectl --context karmada-apiserver apply -f- <<EOF
apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  name: mindspore-1-cpu
spec:
  minAvailable: 1
  schedulerName: volcano
  policies:
    - event: PodEvicted
      action: RestartJob
  plugins:
    ssh: []
    env: []
    svc: []
  maxRetry: 5
  queue: test
  tasks:
    - replicas: 2
      name: "pod"
      template:
        spec:
          containers:
            - image: nginx
              imagePullPolicy: IfNotPresent
              name: mindspore-1-cpu-job
              resources:
                limits:
                  cpu: "0.5"
                  memory: "512Mi"
                requests:
                  cpu: "0.5"
                  memory: "512Mi"
          restartPolicy: OnFailure
---
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: mindspore-1-cpu
spec:
  resourceSelectors:
    - apiVersion: batch.volcano.sh/v1alpha1
      kind: Job
      name: mindspore-1-cpu
  placement:
    replicaScheduling:
      replicaDivisionPreference: Aggregated
      replicaSchedulingType: Duplicated
EOF
```
check rb and vcjob status
```bash
kubectl --context kind-tanber get vcjob                      
No resources found in default namespace.
kubectl --context kind-member1 get vcjob                      
No resources found in default namespace.
kubectl --context karmada-apiserver get rb mindspore-1-cpu-job
NAME                  SCHEDULED   FULLYAPPLIED   AGE
mindspore-1-cpu-job   False                      73s
kubectl --context karmada-apiserver get rb mindspore-1-cpu-job -oyaml
apiVersion: work.karmada.io/v1alpha2
kind: ResourceBinding
metadata:
  annotations:
    propagationpolicy.karmada.io/name: mindspore-1-cpu
    propagationpolicy.karmada.io/namespace: default
  creationTimestamp: "2025-09-04T01:48:33Z"
  finalizers:
  - karmada.io/binding-controller
  generation: 1
  labels:
    propagationpolicy.karmada.io/permanent-id: 97430009-b79c-4547-add2-fc53e670c1e1
    resourcebinding.karmada.io/permanent-id: a8f811d4-039c-4222-a8ba-ce35b1c4dcca
  name: mindspore-1-cpu-job
  namespace: default
  ownerReferences:
  - apiVersion: batch.volcano.sh/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: Job
    name: mindspore-1-cpu
    uid: 4f4e7ad2-394c-4c6b-9f3e-2be8215856de
  resourceVersion: "2655362"
  uid: 74197a1b-35cb-4aef-8146-9ee2bf970401
spec:
  conflictResolution: Abort
  placement:
    clusterTolerations:
    - effect: NoExecute
      key: cluster.karmada.io/not-ready
      operator: Exists
      tolerationSeconds: 300
    - effect: NoExecute
      key: cluster.karmada.io/unreachable
      operator: Exists
      tolerationSeconds: 300
    replicaScheduling:
      replicaDivisionPreference: Aggregated
      replicaSchedulingType: Duplicated
  replicaRequirements:
    resourceRequest:
      cpu: 500m
      memory: 512Mi
  replicas: 2
  resource:
    apiVersion: batch.volcano.sh/v1alpha1
    kind: Job
    name: mindspore-1-cpu
    namespace: default
    resourceVersion: "2655356"
    uid: 4f4e7ad2-394c-4c6b-9f3e-2be8215856de
  schedulerName: default-scheduler
status:
  conditions:
  - lastTransitionTime: "2025-09-04T01:48:33Z"
    message: 'admission webhook "validateresourcebindings.volcano.sh" denied the request:
      Queue <test> is overused when considering ResourceBinding <default/mindspore-1-cpu-job>'
    reason: SchedulerError
    status: "False"
    type: Scheduled
```
+ test vcjob mindspore-1-cpu update replicaSchedulingType to Divided, because total resource request: 0.5C0.5Gi\*2 in queue, equal than queue allocatable, so it can scheduled.
```bash
kubectl --context karmada-apiserver apply -f- <<EOF
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: mindspore-1-cpu
spec:
  resourceSelectors:
    - apiVersion: batch.volcano.sh/v1alpha1
      kind: Job
      name: mindspore-1-cpu
  placement:
    replicaScheduling:
      replicaDivisionPreference: Aggregated
      replicaSchedulingType: Divided
EOF
```
check rb and vcjob status
```bash
kubectl --context karmada-apiserver get rb mindspore-1-cpu-job       
NAME                  SCHEDULED   FULLYAPPLIED   AGE
mindspore-1-cpu-job   True        True           9m33s
kubectl --context kind-member1 get vcjob                             
NAME              STATUS    MINAVAILABLE   RUNNINGS   AGE
mindspore-1-cpu   Running   1              2          15s
kubectl --context kind-tanber get vcjob 
No resources found in default namespace.
```