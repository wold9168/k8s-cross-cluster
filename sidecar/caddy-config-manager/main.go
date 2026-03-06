package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Kubernetes client setup
	config, err := k8sclient.GetConfig()
	if err != nil {
		klog.Error("Authentication failed: ", err.Error())
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("Creating clientset failed: ", err.Error())
		panic(err.Error())
	}

	// Get current namespace
	namespace, err := k8sclient.GetCurrentNamespace()
	if err != nil {
		klog.Warningf("Failed to get current namespace, using default: %v", err)
		namespace = "default"
	}

	klog.Infof("Running in namespace: %s", namespace)

	// Get singleton App instance
	app := GetApp()

	// Initialize the application
	if err := app.Initialize(clientset, namespace); err != nil {
		klog.Error("Initialization failed: ", err.Error())
		panic(err.Error())
	}

	// Setup shutdown handler
	go func() {
		<-sigChan
		klog.Info("Shutdown signal received")
		cancel()
		if err := app.Shutdown(); err != nil {
			klog.Errorf("Shutdown error: %v", err)
		}
	}()

	// Run the application (blocking call)
	if err := app.Run(ctx); err != nil {
		klog.Error("Application error: ", err.Error())
	}
}
