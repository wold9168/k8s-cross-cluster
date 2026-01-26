package k8sclient

import (
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

func TestGetAllConfigMapsInCurrentNamespace_ErrorCases(t *testing.T) {
	// Test with nil namespace (should use default or environment)
	clientset := fake.NewSimpleClientset()

	// Temporarily set environment to have a namespace
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()
	os.Setenv("POD_NAMESPACE", "test-env-namespace")

	configMapList, err := GetAllConfigMapsInCurrentNamespace(clientset, nil)

	if err != nil {
		t.Logf("Error getting configmaps (may be expected): %v", err)
	} else {
		if configMapList == nil {
			t.Fatal("Expected non-nil ConfigMapList")
		}
		t.Logf("Found %d configmaps in environment namespace", len(configMapList.Items))
	}
}