package app

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/klog/v2"

	"volcano.sh/volcano-global/cmd/webhook-manager/app/options"
	"volcano.sh/volcano-global/pkg/webhooks/manager"
	"volcano.sh/volcano-global/pkg/webhooks/resourcebinding/validate"
)

func Run(config *options.Config) error {
	m, err := manager.New(config)
	if err != nil {
		return err
	}

	m.RegisterAdmission(validate.Service)

	webhookServeError := make(chan struct{})
	stopChannel := make(chan os.Signal, 1)
	signal.Notify(stopChannel, syscall.SIGTERM, syscall.SIGINT)

	err = m.Run(webhookServeError)
	if err != nil {
		return err
	}

	select {
	case <-stopChannel:
		klog.Infof("Received SIGTERM, shutting down gracefully")
		return nil
	case <-webhookServeError:
		return fmt.Errorf("unknown webhook server error")
	}
}
