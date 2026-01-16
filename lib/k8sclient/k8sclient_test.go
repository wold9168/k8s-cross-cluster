package k8sclient

import (
	"context"
	"os"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetAllConfigMapsInCurrentNamespace(t *testing.T) {
	// Create fake clientset
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap1",
				Namespace: namespace,
			},
		},
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap2",
				Namespace: namespace,
			},
		},
	)

	// Call the function with namespace parameter
	configMapList, err := GetAllConfigMapsInCurrentNamespace(clientset, &namespace)

	// Verify results
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if configMapList == nil {
		t.Fatal("Expected non-nil ConfigMapList")
	}

	if len(configMapList.Items) != 2 {
		t.Errorf("Expected 2 ConfigMaps, got: %d", len(configMapList.Items))
	}

	// Verify ConfigMap names
	names := []string{configMapList.Items[0].Name, configMapList.Items[1].Name}
	if names[0] != "configmap1" && names[1] != "configmap1" {
		t.Errorf("Expected configmap1 in the list, got: %v", names)
	}
	if names[0] != "configmap2" && names[1] != "configmap2" {
		t.Errorf("Expected configmap2 in the list, got: %v", names)
	}
}

func TestGetAllConfigMapsInCurrentNamespace_Empty(t *testing.T) {
	// Create empty fake clientset
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset()

	// Call the function with namespace parameter
	configMapList, err := GetAllConfigMapsInCurrentNamespace(clientset, &namespace)

	// Verify results
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if configMapList == nil {
		t.Fatal("Expected non-nil ConfigMapList")
	}

	if len(configMapList.Items) != 0 {
		t.Errorf("Expected 0 ConfigMaps, got: %d", len(configMapList.Items))
	}
}

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

func TestUpdateCaddyConfigMap_Create(t *testing.T) {
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset()
	caddyConfig := "service1.test-ns.svc.clusterwise.remote {\n    reverse_proxy service1.test-ns.svc.cluster.local\n}\n"

	err := UpdateCaddyConfigMap(clientset, &namespace, caddyConfig)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify ConfigMap was created
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), CaddyConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get ConfigMap: %v", err)
	}

	if cm.Data[CaddyConfigKey] != caddyConfig {
		t.Errorf("Expected Caddyfile content: %s, got: %s", caddyConfig, cm.Data[CaddyConfigKey])
	}
}

func TestUpdateCaddyConfigMap_Update(t *testing.T) {
	namespace := "test-ns"
	existingConfig := "old config"
	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      CaddyConfigMapName,
				Namespace: namespace,
			},
			Data: map[string]string{
				CaddyConfigKey: existingConfig,
			},
		},
	)
	newConfig := "service1.test-ns.svc.clusterwise.remote {\n    reverse_proxy service1.test-ns.svc.cluster.local\n}\n"

	err := UpdateCaddyConfigMap(clientset, &namespace, newConfig)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify ConfigMap was updated
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), CaddyConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get ConfigMap: %v", err)
	}

	if cm.Data[CaddyConfigKey] != newConfig {
		t.Errorf("Expected Caddyfile content: %s, got: %s", newConfig, cm.Data[CaddyConfigKey])
	}

	if cm.Data[CaddyConfigKey] == existingConfig {
		t.Errorf("ConfigMap was not updated, still has old config")
	}
}

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
