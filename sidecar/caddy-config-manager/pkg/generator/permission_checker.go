package generator

import (
	"context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
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
	ns := pc.namespace
	if ns == "" {
		var err error
		ns, err = k8sclient.GetCurrentNamespace()
		if err != nil {
			klog.Warningf("Failed to get current namespace: %v", err)
			ns = "default"
		}
	}

	klog.Infof("Checking permissions in namespace: %s", ns)

	if err := k8sclient.CheckServicePermissions(pc.clientset, ctx, ns); err != nil {
		return err
	}

	klog.Infof("All required permissions verified in namespace: %s", ns)
	return nil
}
