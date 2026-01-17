package k8sclient

import (
	"net"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetLocalIP(t *testing.T) {
	ip, err := getLocalIP()

	if err != nil {
		// In some test environments, there might not be suitable network interfaces
		t.Logf("getLocalIP failed (expected in some test environments): %v", err)
	} else {
		// Validate that the returned IP is a valid IP address
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			t.Errorf("Returned IP is not valid: %s", ip)
		}
		// Ensure it's not a loopback address in most cases
		parsedIP = net.ParseIP(ip)
		if parsedIP != nil && parsedIP.IsLoopback() {
			t.Logf("Returned loopback IP: %s", ip)
		}
	}
}

func TestGetPodNameFromCgroup(t *testing.T) {
	// This function tries to read from /proc/self/cgroup which doesn't exist in test environment
	// So it should return an error
	podName, err := getPodNameFromCgroup()

	if err == nil {
		t.Errorf("Expected error when reading from non-existent cgroup file, got nil")
	}
	if podName != "" {
		t.Errorf("Expected empty pod name, got: %s", podName)
	}

	// In test environments, this function will typically return an error
	// which is the expected behavior when not running in a container
}

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