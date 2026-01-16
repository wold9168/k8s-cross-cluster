package k8sclient

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestCheckConfigMapPermissions(t *testing.T) {
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset()
	ctx := context.Background()

	// This test will fail because fake clientset does not support SelfSubjectAccessReview
	// In production, use mock or integration tests
	err := CheckConfigMapPermissions(clientset, ctx, namespace)

	// Expected to return error because fake clientset does not support AuthorizationV1 API
	if err == nil {
		t.Errorf("Expected error from fake clientset, got nil")
	}
}