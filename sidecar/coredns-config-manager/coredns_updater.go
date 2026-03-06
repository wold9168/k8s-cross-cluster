package main

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
)

// CoreDNSUpdater manages CoreDNS configuration updates
type CoreDNSUpdater struct {
	clientset kubernetes.Interface
	config    CoreDNSConfig
	corefileManager *CorefileManager
}

// NewCoreDNSUpdater creates a new CoreDNSUpdater
func NewCoreDNSUpdater(clientset kubernetes.Interface, config CoreDNSConfig) *CoreDNSUpdater {
	return &CoreDNSUpdater{
		clientset: clientset,
		config:    config,
		corefileManager: NewCorefileManager(config),
	}
}

// EnsureConfig ensures CoreDNS configuration is properly set up with upstream server
func (cu *CoreDNSUpdater) EnsureConfig(ctx context.Context, upstreamServer string) error {
	// Get current CoreDNS ConfigMap
	coreDNSCM, err := k8sclient.GetConfigMap(cu.clientset, &cu.config.Namespace, cu.config.ConfigMapName)
	if err != nil {
		return fmt.Errorf("failed to get CoreDNS ConfigMap: %w", err)
	}

	// Get current Corefile content
	currentCorefile, exists := coreDNSCM.Data[cu.config.ConfigKey]
	if !exists {
		return fmt.Errorf("Corefile key '%s' does not exist in ConfigMap", cu.config.ConfigKey)
	}

	// Validate current Corefile
	if err := cu.corefileManager.ValidateCorefile(currentCorefile); err != nil {
		klog.Warningf("Corefile validation warning: %v", err)
	}

	// Check if update is needed
	if !cu.corefileManager.NeedsUpdate(currentCorefile, upstreamServer) {
		klog.V(4).Info("CoreDNS configuration is already up to date")
		return nil
	}

	// Update Corefile content
	updatedCorefile := cu.corefileManager.UpdateCorefile(currentCorefile, upstreamServer)

	// Update ConfigMap
	coreDNSCM.Data[cu.config.ConfigKey] = updatedCorefile
	_, err = k8sclient.UpdateExistingConfigMap(cu.clientset, &cu.config.Namespace, coreDNSCM)
	if err != nil {
		return fmt.Errorf("failed to update CoreDNS ConfigMap: %w", err)
	}

	// Rollout CoreDNS deployment
	if err := cu.rolloutDeployment(ctx); err != nil {
		return fmt.Errorf("failed to rollout CoreDNS deployment: %w", err)
	}

	klog.Infof("Successfully updated CoreDNS configuration to forward *.remote queries to %s", upstreamServer)
	return nil
}

// rolloutDeployment triggers a rollout of the CoreDNS deployment
func (cu *CoreDNSUpdater) rolloutDeployment(ctx context.Context) error {
	err := k8sclient.RolloutDeployment(cu.clientset, ctx, cu.config.Namespace, cu.config.DeploymentName)
	
	// Retry with backoff on failure
	retryCount := 0
	maxRetries := 5
	for err != nil && retryCount < maxRetries {
		klog.ErrorS(err, "Failed to rollout CoreDNS", "retry", retryCount+1)
		time.Sleep(10 * time.Second)
		err = k8sclient.RolloutDeployment(cu.clientset, ctx, cu.config.Namespace, cu.config.DeploymentName)
		retryCount++
	}

	return err
}

// GetConfig returns the current CoreDNS configuration
func (cu *CoreDNSUpdater) GetConfig() CoreDNSConfig {
	return cu.config
}
