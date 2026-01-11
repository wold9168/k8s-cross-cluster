package test

import (
	"context"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	k8sclient "github.com/wold9168/k8s-cross-cluster/sidecar/caddy-config-manager/pkg/k8sclient"
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
	configMapList, err := k8sclient.GetAllConfigMapsInCurrentNamespace(clientset, &namespace)

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
	configMapList, err := k8sclient.GetAllConfigMapsInCurrentNamespace(clientset, &namespace)

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
	serviceList, err := k8sclient.GetAllServicesInCurrentNamespace(clientset, &namespace)

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
	serviceList, err := k8sclient.GetAllServicesInCurrentNamespace(clientset, &namespace)

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

	err := k8sclient.UpdateCaddyConfigMap(clientset, &namespace, caddyConfig)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify ConfigMap was created
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), k8sclient.CaddyConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get ConfigMap: %v", err)
	}

	if cm.Data[k8sclient.CaddyConfigKey] != caddyConfig {
		t.Errorf("Expected Caddyfile content: %s, got: %s", caddyConfig, cm.Data[k8sclient.CaddyConfigKey])
	}
}

func TestUpdateCaddyConfigMap_Update(t *testing.T) {
	namespace := "test-ns"
	existingConfig := "old config"
	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      k8sclient.CaddyConfigMapName,
				Namespace: namespace,
			},
			Data: map[string]string{
				k8sclient.CaddyConfigKey: existingConfig,
			},
		},
	)
	newConfig := "service1.test-ns.svc.clusterwise.remote {\n    reverse_proxy service1.test-ns.svc.cluster.local\n}\n"

	err := k8sclient.UpdateCaddyConfigMap(clientset, &namespace, newConfig)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify ConfigMap was updated
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), k8sclient.CaddyConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get ConfigMap: %v", err)
	}

	if cm.Data[k8sclient.CaddyConfigKey] != newConfig {
		t.Errorf("Expected Caddyfile content: %s, got: %s", newConfig, cm.Data[k8sclient.CaddyConfigKey])
	}

	if cm.Data[k8sclient.CaddyConfigKey] == existingConfig {
		t.Errorf("ConfigMap was not updated, still has old config")
	}
}

func TestCheckPermissions(t *testing.T) {
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset()

	// This test will fail because fake clientset does not support SelfSubjectAccessReview
	// In production, use mock or integration tests
	err := k8sclient.CheckPermissions(clientset, &namespace)

	// Expected to return error because fake clientset does not support AuthorizationV1 API
	if err == nil {
		t.Errorf("Expected error from fake clientset, got nil")
	}
}

func TestCheckPermissions_NilNamespace(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	// Test nil namespace parameter, should use default namespace retrieval logic
	err := k8sclient.CheckPermissions(clientset, nil)

	// Expected to return error because fake clientset does not support AuthorizationV1 API
	if err == nil {
		t.Errorf("Expected error from fake clientset, got nil")
	}
}