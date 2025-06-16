package dispatchplugin

import (
	"k8s.io/client-go/kubernetes"
	"os"
	"testing"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"k8s.io/client-go/tools/clientcmd"
	vcclient "volcano.sh/apis/pkg/client/clientset/versioned"

	e2eutil "volcano.sh/volcano-global/test/e2e/util"
)

func TestMain(m *testing.M) {
	home := e2eutil.HomeDir()
	configPath := e2eutil.KubeconfigPath(home)
	config, _ := clientcmd.BuildConfigFromFlags(e2eutil.MasterURL(), configPath)
	e2eutil.KubeClient = kubernetes.NewForConfigOrDie(config)
	e2eutil.VcClient = vcclient.NewForConfigOrDie(config)
	e2eutil.KmClient = karmadaclientset.NewForConfigOrDie(config)
	os.Exit(m.Run())
}
