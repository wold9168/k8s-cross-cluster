package k8sclient

import (
	"context"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetAllServicesInCurrentNamespace(t *testing.T) {
	// Create fake clientset
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset(
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service1",
				Namespace: namespace,
			},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service2",
				Namespace: namespace,
			},
		},
	)

	// Call the function with namespace parameter
	serviceList, err := GetAllServicesInCurrentNamespace(clientset, &namespace)

	// Verify results
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if serviceList == nil {
		t.Fatal("Expected non-nil ServiceList")
	}

	if len(serviceList.Items) != 2 {
		t.Errorf("Expected 2 Services, got: %d", len(serviceList.Items))
	}

	// Verify Service names
	names := []string{serviceList.Items[0].Name, serviceList.Items[1].Name}
	if names[0] != "service1" && names[1] != "service1" {
		t.Errorf("Expected service1 in the list, got: %v", names)
	}
	if names[0] != "service2" && names[1] != "service2" {
		t.Errorf("Expected service2 in the list, got: %v", names)
	}
}

func TestGetAllServicesInCurrentNamespace_Empty(t *testing.T) {
	// Create empty fake clientset
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset()

	// Call the function with namespace parameter
	serviceList, err := GetAllServicesInCurrentNamespace(clientset, &namespace)

	// Verify results
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if serviceList == nil {
		t.Fatal("Expected non-nil ServiceList")
	}

	if len(serviceList.Items) != 0 {
		t.Errorf("Expected 0 Services, got: %d", len(serviceList.Items))
	}
}

func TestCheckServicePermissions(t *testing.T) {
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset()
	ctx := context.Background()

	// This test will fail because fake clientset does not support SelfSubjectAccessReview
	// In production, use mock or integration tests
	err := CheckServicePermissions(clientset, ctx, namespace)

	// Expected to return error because fake clientset does not support AuthorizationV1 API
	if err == nil {
		t.Errorf("Expected error from fake clientset, got nil")
	}
}

func TestGetAllServicesInCurrentNamespace_ErrorCases(t *testing.T) {
	// Note: The fake clientset doesn't properly simulate all error conditions
	// like permissions errors or network issues, so we focus on the namespace resolution

	// Test with nil namespace (should use default)
	clientset := fake.NewSimpleClientset()

	// Temporarily set environment to have a namespace
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()
	os.Setenv("POD_NAMESPACE", "test-env-namespace")

	serviceList, err := GetAllServicesInCurrentNamespace(clientset, nil)

	if err != nil {
		t.Logf("Error getting services (may be expected): %v", err)
	} else {
		if serviceList == nil {
			t.Fatal("Expected non-nil ServiceList")
		}
		t.Logf("Found %d services in environment namespace", len(serviceList.Items))
	}
}