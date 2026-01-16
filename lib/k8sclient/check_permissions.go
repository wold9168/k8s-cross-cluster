package k8sclient

import (
	"context"
	"fmt"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// CheckConfigMapPermissions verifies ConfigMap permissions
func CheckConfigMapPermissions(clientset kubernetes.Interface, ctx context.Context, namespace string) error {
	// Check ConfigMaps read permissions (get, list)
	if err := checkResourcePermission(clientset, ctx, namespace, "configmaps", "get"); err != nil {
		return fmt.Errorf("missing ConfigMaps get permission: %w", err)
	}
	if err := checkResourcePermission(clientset, ctx, namespace, "configmaps", "list"); err != nil {
		return fmt.Errorf("missing ConfigMaps list permission: %w", err)
	}

	// Check ConfigMaps write permissions (create, update)
	if err := checkResourcePermission(clientset, ctx, namespace, "configmaps", "create"); err != nil {
		return fmt.Errorf("missing ConfigMaps create permission: %w", err)
	}
	if err := checkResourcePermission(clientset, ctx, namespace, "configmaps", "update"); err != nil {
		return fmt.Errorf("missing ConfigMaps update permission: %w", err)
	}

	return nil
}

// CheckServicePermissions verifies Service permissions
func CheckServicePermissions(clientset kubernetes.Interface, ctx context.Context, namespace string) error {
	// Check Services read permissions (list)
	if err := checkResourcePermission(clientset, ctx, namespace, "services", "list"); err != nil {
		return fmt.Errorf("missing Services list permission: %w", err)
	}

	return nil
}

// CheckPodPermissions verifies Pod permissions
func CheckPodPermissions(clientset kubernetes.Interface, ctx context.Context, namespace string) error {
	// Check Pods read permissions (get)
	if err := checkResourcePermission(clientset, ctx, namespace, "pods", "get"); err != nil {
		return fmt.Errorf("missing Pods get permission: %w", err)
	}

	return nil
}

// checkResourcePermission checks if the current user has permission to perform a verb on a resource
func checkResourcePermission(clientset kubernetes.Interface, ctx context.Context, namespace, resource, verb string) error {
	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      verb,
				Group:     "",
				Resource:  resource,
			},
		},
	}

	response, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create SelfSubjectAccessReview: %w", err)
	}

	if !response.Status.Allowed {
		return fmt.Errorf("access denied for %s %s in namespace %s", verb, resource, namespace)
	}

	klog.Infof("Permission check passed: %s %s in namespace %s", verb, resource, namespace)
	return nil
}
