package main

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"

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
		return fmt.Errorf("ConfigMap permission check failed: %w", err)
	}
	if err := k8sclient.CheckPodPermissions(pc.clientset, ctx, pc.namespace); err != nil {
		return fmt.Errorf("Pod permission check failed: %w", err)
	}
	return nil
}
