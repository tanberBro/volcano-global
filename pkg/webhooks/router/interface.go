package router

import (
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	informerclusterv1alpha1 "github.com/karmada-io/karmada/pkg/generated/informers/externalversions/cluster/v1alpha1"
	informerworkv1aplha2 "github.com/karmada-io/karmada/pkg/generated/informers/externalversions/work/v1alpha2"
	admissionv1 "k8s.io/api/admission/v1"
	whv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"sync"
	volcanoclientset "volcano.sh/apis/pkg/client/clientset/versioned"
	schedulinginformer "volcano.sh/apis/pkg/client/informers/externalversions/scheduling/v1beta1"
	cache2 "volcano.sh/volcano-global/pkg/webhooks/cache"
)

// The AdmitFunc returns response.
type AdmitFunc func(admissionv1.AdmissionReview, *cache2.DispatcherCacheSnapshot) *admissionv1.AdmissionResponse
type AdmissionHandler func(w http.ResponseWriter, r *http.Request)

type AdmissionServiceConfig struct {
	mutex          sync.Mutex
	SchedulerNames []string
	KubeClient     kubernetes.Interface
	DynamicClient  dynamic.DynamicClient
	VolcanoClient  volcanoclientset.Interface
	KarmadaClient  karmadaclientset.Interface

	queueInformer           schedulinginformer.QueueInformer
	resourceBindingInformer informerworkv1aplha2.ResourceBindingInformer
	clusterInformer         informerclusterv1alpha1.ClusterInformer
}

type AdmissionService struct {
	Path    string
	Func    AdmitFunc
	Handler AdmissionHandler

	cache2.WebhookCacheInterface

	ValidatingConfig *whv1.ValidatingWebhookConfiguration
	MutatingConfig   *whv1.MutatingWebhookConfiguration

	Config *AdmissionServiceConfig
}
