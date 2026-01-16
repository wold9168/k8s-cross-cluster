package k8sclient

import (
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetCurrentPodIP_FromK8sAPI(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodIP := os.Getenv("POD_IP")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_IP", origPodIP)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set test environment
	podName := "test-pod"
	namespace := "test-ns"
	podIP := "10.244.1.5"
	os.Setenv("POD_NAME", podName)
	os.Setenv("POD_NAMESPACE", namespace)
	os.Unsetenv("POD_IP")

	// Create fake clientset with test pod
	clientset := fake.NewSimpleClientset(
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
			},
			Status: v1.PodStatus{
				PodIP: podIP,
			},
		},
	)

	// Call function
	ip, err := GetCurrentPodIP(clientset)

	// Verify results
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if ip != podIP {
		t.Errorf("Expected IP %s, got: %s", podIP, ip)
	}
}

func TestGetCurrentPodIP_FromEnvVar(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodIP := os.Getenv("POD_IP")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_IP", origPodIP)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set test environment - POD_IP only
	podIP := "10.244.2.10"
	os.Unsetenv("POD_NAME")
	os.Unsetenv("POD_NAMESPACE")
	os.Setenv("POD_IP", podIP)

	// Create empty fake clientset (API will fail)
	clientset := fake.NewSimpleClientset()

	// Call function
	ip, err := GetCurrentPodIP(clientset)

	// Verify results
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if ip != podIP {
		t.Errorf("Expected IP %s, got: %s", podIP, ip)
	}
}

func TestGetCurrentPodIP_NoPodName(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodIP := os.Getenv("POD_IP")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_IP", origPodIP)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set test environment - no env vars
	os.Unsetenv("POD_NAME")
	os.Unsetenv("POD_IP")
	os.Unsetenv("POD_NAMESPACE")

	// Create empty fake clientset
	clientset := fake.NewSimpleClientset()

	// Call function - should try to get local IP
	ip, err := GetCurrentPodIP(clientset)

	// Should either get local IP or error, but not panic
	if ip == "" && err != nil {
		// This is acceptable - might not have a suitable network interface in test env
		t.Logf("No local IP found in test environment: %v", err)
	} else if ip != "" && err == nil {
		t.Logf("Got local IP: %s", ip)
	}
}

func TestGetPodIPFromK8sAPI_Success(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	podName := "test-pod"
	namespace := "test-ns"
	podIP := "10.244.1.10"
	os.Setenv("POD_NAME", podName)
	os.Setenv("POD_NAMESPACE", namespace)

	clientset := fake.NewSimpleClientset(
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
			},
			Status: v1.PodStatus{
				PodIP: podIP,
			},
		},
	)

	ip, err := getPodIPFromK8sAPI(clientset)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if ip != podIP {
		t.Errorf("Expected IP %s, got: %s", podIP, ip)
	}
}

func TestGetPodIPFromK8sAPI_NoPodName(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	os.Unsetenv("POD_NAME")
	os.Unsetenv("POD_NAMESPACE")

	clientset := fake.NewSimpleClientset()

	_, err := getPodIPFromK8sAPI(clientset)

	if err == nil {
		t.Error("Expected error when POD_NAME is not set, got nil")
	}
}

func TestGetPodIPFromK8sAPI_PodNotFound(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	os.Setenv("POD_NAME", "nonexistent-pod")
	os.Setenv("POD_NAMESPACE", "test-ns")

	clientset := fake.NewSimpleClientset()

	_, err := getPodIPFromK8sAPI(clientset)

	if err == nil {
		t.Error("Expected error when pod is not found, got nil")
	}
}

func TestGetPodIPFromK8sAPI_NoIPAssigned(t *testing.T) {
	// Save original environment variables
	origPodName := os.Getenv("POD_NAME")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAME", origPodName)
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	podName := "test-pod"
	namespace := "test-ns"
	os.Setenv("POD_NAME", podName)
	os.Setenv("POD_NAMESPACE", namespace)

	// Pod with no IP assigned
	clientset := fake.NewSimpleClientset(
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
			},
			Status: v1.PodStatus{
				PodIP: "",
			},
		},
	)

	_, err := getPodIPFromK8sAPI(clientset)

	if err == nil {
		t.Error("Expected error when pod has no IP assigned, got nil")
	}
}