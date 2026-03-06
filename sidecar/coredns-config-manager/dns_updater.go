package main

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
)

// ensureCoreDNSConfig checks and updates CoreDNS configuration
// Deprecated: Use CoreDNSUpdater.EnsureConfig instead
func ensureCoreDNSConfig(clientset *kubernetes.Clientset, upstreamServer string) error {
	config := CoreDNSConfig{
		Namespace:      CoreDNSNamespace,
		ConfigMapName:  CoreDNSConfigMapName,
		ConfigKey:      CoreDNSConfigKey,
		DeploymentName: CoreDNSDeploymentName,
		ManagedSection: ManagedSection{
			StartMarker: ManagedSectionStart,
			EndMarker:   ManagedSectionEnd,
		},
	}

	updater := NewCoreDNSUpdater(clientset, config)
	ctx := context.Background()
	return updater.EnsureConfig(ctx, upstreamServer)
}

// authorization checks permissions for ConfigMaps and Pods
// Deprecated: Use PermissionCheckerImpl instead
func authorization(clientset *kubernetes.Clientset, ctx context.Context) error {
	ns, err := k8sclient.GetCurrentNamespace()
	if err != nil {
		return fmt.Errorf("failed to get namespace: %w", err)
	}

	if err := k8sclient.CheckConfigMapPermissions(clientset, ctx, ns); err != nil {
		return fmt.Errorf("ConfigMap permission check failed: %w", err)
	}

	if err := k8sclient.CheckPodPermissions(clientset, ctx, ns); err != nil {
		return fmt.Errorf("Pod permission check failed: %w", err)
	}

	return nil
}
