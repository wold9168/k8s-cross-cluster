package main

import (
	"context"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
	"github.com/wold9168/k8s-cross-cluster/sidecar/caddy-config-manager/pkg/generator"
)

// PermissionChecker validates Kubernetes API permissions
type PermissionChecker struct {
	clientset kubernetes.Interface
	namespace string
}

// NewPermissionChecker creates a new PermissionChecker
func NewPermissionChecker(clientset kubernetes.Interface, namespace string) *PermissionChecker {
	return &PermissionChecker{
		clientset: clientset,
		namespace: namespace,
	}
}

// CheckServicePermissions verifies read access to Services
func (pc *PermissionChecker) CheckServicePermissions(ctx context.Context) error {
	return k8sclient.CheckServicePermissions(pc.clientset, ctx, pc.namespace)
}

// ConfigSyncer orchestrates periodic configuration synchronization
type ConfigSyncer struct {
	configManager *generator.ConfigManager
	permissionChecker *PermissionChecker
	interval      time.Duration
}

// NewConfigSyncer creates a new ConfigSyncer
func NewConfigSyncer(configManager *generator.ConfigManager, permissionChecker *PermissionChecker, interval time.Duration) *ConfigSyncer {
	return &ConfigSyncer{
		configManager:     configManager,
		permissionChecker: permissionChecker,
		interval:          interval,
	}
}

// Run starts the configuration synchronization loop
func (cs *ConfigSyncer) Run(ctx context.Context) {
	for {
		// Check permissions
		if err := cs.permissionChecker.CheckServicePermissions(ctx); err != nil {
			klog.Errorf("Permission check failed: %v, retrying in %v", err, cs.interval)
			time.Sleep(cs.interval)
			continue
		}

		// Sync configuration
		if err := cs.configManager.Sync(ctx); err != nil {
			klog.Errorf("Configuration sync failed: %v", err)
		}

		time.Sleep(cs.interval)
	}
}

func main() {
	ctx := context.Background()

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

	// Initialize components
	configManager := generator.NewConfigManager(
		clientset,
		generator.WithConfigPath("/config/Caddyfile"),
		generator.WithConfigDir("/config"),
	)

	// Load cluster name
	if err := configManager.LoadClusterName("tailscale-cluster-name"); err != nil {
		klog.Warningf("Failed to load cluster name: %v", err)
	}

	permissionChecker := NewPermissionChecker(clientset, namespace)

	syncer := NewConfigSyncer(configManager, permissionChecker, 10*time.Second)

	klog.Infof("Starting Caddy config syncer (interval: 10s)")
	syncer.Run(ctx)
}
