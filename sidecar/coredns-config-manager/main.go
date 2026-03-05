package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
)

// PermissionCheckerImpl validates Kubernetes API permissions
type PermissionCheckerImpl struct {
	clientset *kubernetes.Clientset
	namespace string
}

// NewPermissionChecker creates a new PermissionCheckerImpl
func NewPermissionChecker(clientset *kubernetes.Clientset, namespace string) *PermissionCheckerImpl {
	return &PermissionCheckerImpl{
		clientset: clientset,
		namespace: namespace,
	}
}

// Check verifies permissions for ConfigMaps and Pods
func (pc *PermissionCheckerImpl) Check(ctx context.Context) error {
	if err := k8sclient.CheckConfigMapPermissions(pc.clientset, ctx, pc.namespace); err != nil {
		return err
	}
	if err := k8sclient.CheckPodPermissions(pc.clientset, ctx, pc.namespace); err != nil {
		return err
	}
	return nil
}

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

	// Initialize DNSConfigManager
	dnsConfigManager := NewDNSConfigManager(clientset, DefaultDNSConfigManagerConfig())

	// Initialize components
	if err := dnsConfigManager.Initialize(ctx); err != nil {
		klog.Error("Initialization failed: ", err.Error())
		panic(err.Error())
	}

	// Setup shutdown handler
	go func() {
		<-sigChan
		klog.Info("Shutdown signal received")
		cancel()
		if err := dnsConfigManager.Shutdown(); err != nil {
			klog.Errorf("Shutdown error: %v", err)
		}
	}()

	// Permission check loop and sync
	permissionChecker := NewPermissionChecker(clientset, namespace)

	klog.Info("Starting DNS configuration manager")
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("Shutting down gracefully")
			return
		case <-ticker.C:
			// Check permissions
			if err := permissionChecker.Check(ctx); err != nil {
				klog.Errorf("Authorization failed: %v, retrying in %v", err, syncInterval)
				continue
			}

			// Perform sync
			if err := dnsConfigManager.Sync(ctx); err != nil {
				klog.Errorf("Sync failed: %v", err)
			}
		}
	}
}
