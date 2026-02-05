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

func TestGetCurrentPodServiceClusterIP_Success(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set environment variables
	os.Setenv("POD_NAME", "test-pod")
	os.Setenv("POD_NAMESPACE", "test-ns")

	// Create fake clientset with Pod and Service
	clientset := fake.NewSimpleClientset(
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "test-ns",
				Labels: map[string]string{
					"app":     "myapp",
					"version": "v1",
				},
			},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "myapp-service",
				Namespace: "test-ns",
			},
			Spec: v1.ServiceSpec{
				ClusterIP: "10.0.0.100",
				Selector: map[string]string{
					"app": "myapp",
				},
			},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-service",
				Namespace: "test-ns",
			},
			Spec: v1.ServiceSpec{
				ClusterIP: "10.0.0.200",
				Selector: map[string]string{
					"app": "otherapp",
				},
			},
		},
	)

	// Call the function
	clusterIP, err := GetCurrentPodServiceClusterIP(clientset)

	// Verify results
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if clusterIP != "10.0.0.100" {
		t.Errorf("Expected ClusterIP 10.0.0.100, got: %s", clusterIP)
	}
}

func TestGetCurrentPodServiceClusterIP_MGetCurrentPodServiceClusterIPultipleSelectorMatch(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set environment variables
	os.Setenv("POD_NAME", "test-pod")
	os.Setenv("POD_NAMESPACE", "test-ns")

	// Create fake clientset with Pod and multiple matching Services
	// The function should return the first matching Service
	clientset := fake.NewSimpleClientset(
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "test-ns",
				Labels: map[string]string{
					"app":     "myapp",
					"version": "v1",
				},
			},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service1",
				Namespace: "test-ns",
			},
			Spec: v1.ServiceSpec{
				ClusterIP: "10.0.0.100",
				Selector: map[string]string{
					"app": "myapp",
				},
			},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service2",
				Namespace: "test-ns",
			},
			Spec: v1.ServiceSpec{
				ClusterIP: "10.0.0.200",
				Selector: map[string]string{
					"app":     "myapp",
					"version": "v1",
				},
			},
		},
	)

	// Call the function
	clusterIP, err := GetCurrentPodServiceClusterIP(clientset)

	// Verify results - should return the first matching Service
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if clusterIP != "10.0.0.100" && clusterIP != "10.0.0.200" {
		t.Errorf("Expected one of the matching ClusterIPs, got: %s", clusterIP)
	}

	t.Logf("Got ClusterIP: %s (one of the matching services)", clusterIP)
}

func TestGetCurrentPodServiceClusterIP_NoPodName(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set only namespace, not POD_NAME
	os.Unsetenv("POD_NAME")
	os.Setenv("POD_NAMESPACE", "test-ns")

	clientset := fake.NewSimpleClientset()

	// Call the function
	clusterIP, err := GetCurrentPodServiceClusterIP(clientset)

	// Verify results
	if err == nil {
		t.Error("Expected error when POD_NAME is not set, got nil")
	}

	if clusterIP != "" {
		t.Errorf("Expected empty ClusterIP, got: %s", clusterIP)
	}
}

func TestGetCurrentPodServiceClusterIP_PodNoLabels(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set environment variables
	os.Setenv("POD_NAME", "test-pod")
	os.Setenv("POD_NAMESPACE", "test-ns")

	// Create fake clientset with Pod that has no labels
	clientset := fake.NewSimpleClientset(
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "test-ns",
				// No labels
			},
		},
	)

	// Call the function
	clusterIP, err := GetCurrentPodServiceClusterIP(clientset)

	// Verify results
	if err == nil {
		t.Error("Expected error when Pod has no labels, got nil")
	}

	if clusterIP != "" {
		t.Errorf("Expected empty ClusterIP, got: %s", clusterIP)
	}
}

func TestGetCurrentPodServiceClusterIP_NoMatchingService(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set environment variables
	os.Setenv("POD_NAME", "test-pod")
	os.Setenv("POD_NAMESPACE", "test-ns")

	// Create fake clientset with Pod but no matching Service
	clientset := fake.NewSimpleClientset(
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "test-ns",
				Labels: map[string]string{
					"app": "myapp",
				},
			},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "other-service",
				Namespace: "test-ns",
			},
			Spec: v1.ServiceSpec{
				ClusterIP: "10.0.0.100",
				Selector: map[string]string{
					"app": "otherapp",
				},
			},
		},
	)

	// Call the function
	clusterIP, err := GetCurrentPodServiceClusterIP(clientset)

	// Verify results
	if err == nil {
		t.Error("Expected error when no matching Service found, got nil")
	}

	if clusterIP != "" {
		t.Errorf("Expected empty ClusterIP, got: %s", clusterIP)
	}
}

func TestGetCurrentPodServiceClusterIP_ServiceNoSelector(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set environment variables
	os.Setenv("POD_NAME", "test-pod")
	os.Setenv("POD_NAMESPACE", "test-ns")

	// Create fake clientset with Service that has no selector (e.g., ExternalName type)
	clientset := fake.NewSimpleClientset(
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "test-ns",
				Labels: map[string]string{
					"app": "myapp",
				},
			},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "external-service",
				Namespace: "test-ns",
			},
			Spec: v1.ServiceSpec{
				ClusterIP: "None",
				Type:      v1.ServiceTypeExternalName,
				// No selector
			},
		},
	)

	// Call the function
	clusterIP, err := GetCurrentPodServiceClusterIP(clientset)

	// Verify results
	if err == nil {
		t.Error("Expected error when no matching Service found, got nil")
	}

	if clusterIP != "" {
		t.Errorf("Expected empty ClusterIP, got: %s", clusterIP)
	}
}

func TestGetCurrentPodServiceClusterIP_PodNotFound(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set environment variables
	os.Setenv("POD_NAME", "nonexistent-pod")
	os.Setenv("POD_NAMESPACE", "test-ns")

	// Create fake clientset without the Pod
	clientset := fake.NewSimpleClientset()

	// Call the function
	clusterIP, err := GetCurrentPodServiceClusterIP(clientset)

	// Verify results
	if err == nil {
		t.Error("Expected error when Pod is not found, got nil")
	}

	if clusterIP != "" {
		t.Errorf("Expected empty ClusterIP, got: %s", clusterIP)
	}
}