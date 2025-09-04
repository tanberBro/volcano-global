package manager

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"volcano.sh/volcano/pkg/kube"

	"volcano.sh/volcano-global/cmd/webhook-manager/app/options"
	"volcano.sh/volcano-global/pkg/webhooks/cache"
	"volcano.sh/volcano-global/pkg/webhooks/router"
)

type Manager struct {
	config        *options.Config
	restConfig    *rest.Config
	kubeClient    *kubernetes.Clientset
	dynamicClient dynamic.Interface
	admissionMap  map[string]*router.AdmissionService
	cache         cache.WebhookCacheInterface
}

func New(config *options.Config) (*Manager, error) {
	restConfig, err := kube.BuildConfig(config.KubeClientOptions)
	if err != nil {
		return nil, fmt.Errorf("unable to build k8s config: %v", err)
	}
	cacheOption := &cache.WebhookCacheOption{
		DefaultQueueName: config.DefaultQueueName,
		RestConfig:       restConfig,
	}
	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to build k8s client: %v", err)
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to build k8s dynamic client: %v", err)
	}
	return &Manager{
		config:        config,
		restConfig:    restConfig,
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		admissionMap:  make(map[string]*router.AdmissionService),
		cache:         cache.NewWebhookCache(cacheOption),
	}, nil
}

func (m *Manager) Run(webhookServeError chan struct{}) error {
	m.start(webhookServeError)
	return m.runWebhook(webhookServeError)
}

func (m *Manager) start(stopCh <-chan struct{}) {
	m.cache.Run(stopCh)
}

func (m *Manager) RegisterAdmission(service *router.AdmissionService) {
	if _, found := m.admissionMap[service.Path]; found {
		klog.Fatalf("duplicated admission service for %s", service.Path)
	}

	// Also register handler to the service.
	service.Handler = func(w http.ResponseWriter, r *http.Request) {
		m.serve(w, r, service.Func)
	}

	m.admissionMap[service.Path] = service

	return
}

func (m *Manager) runWebhook(webhookServeError chan struct{}) error {
	for _, service := range m.admissionMap {
		http.HandleFunc(service.Path, service.Handler)
		err := addCaCertForWebhook(m.kubeClient, service, m.config.CaCertData)
		if err != nil {
			return err
		}
	}
	server := &http.Server{
		Addr:      m.config.ListenAddress + ":" + strconv.Itoa(m.config.Port),
		TLSConfig: configTLS(m.config, m.restConfig),
	}
	go func() {
		err := server.ListenAndServeTLS("", "")
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			close(webhookServeError)
			klog.Fatalf("ListenAndServeTLS for admission webhook failed: %v", err)
		}
		klog.Info("Volcano-global Webhook manager started.")
	}()
	return nil
}
