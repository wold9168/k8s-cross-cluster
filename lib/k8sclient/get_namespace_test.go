package k8sclient

import (
	"os"
	"testing"
)

func TestGetCurrentNamespace_FromEnvVar(t *testing.T) {
	// Save original environment variable
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set test environment variable
	testNamespace := "test-namespace-from-env"
	os.Setenv("POD_NAMESPACE", testNamespace)

	// Call function
	namespace, err := GetCurrentNamespace()

	// Verify results
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if namespace != testNamespace {
		t.Errorf("Expected namespace %s, got: %s", testNamespace, namespace)
	}
}

func TestGetCurrentNamespace_Default(t *testing.T) {
	// Save original environment variable
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Unset the environment variable to trigger default behavior
	os.Unsetenv("POD_NAMESPACE")

	// Call function
	namespace, err := GetCurrentNamespace()

	// Verify results - should return "default" when no environment variable is set
	// and no service account file exists (which won't exist in test environment)
	if err == nil {
		// If no error, it means it found a namespace from kubeconfig
		t.Logf("Found namespace from kubeconfig: %s", namespace)
	} else {
		// If there's an error (likely due to missing kubeconfig), it should return "default"
		if namespace != "default" {
			t.Errorf("Expected default namespace 'default', got: %s", namespace)
		}
	}
}

func TestGetNamespaceFromKubeconfig_MissingFile(t *testing.T) {
	// This test verifies the behavior when kubeconfig file doesn't exist
	// The function should return "default" and an error
	
	// Temporarily change home directory to a non-existent path to ensure no kubeconfig exists
	origHome := os.Getenv("HOME")
	defer func() {
		os.Setenv("HOME", origHome)
	}()
	
	tempHome := "/tmp/nonexistent-home-dir"
	os.Setenv("HOME", tempHome)
	
	namespace, err := getNamespaceFromKubeconfig()
	
	if namespace != "default" {
		t.Errorf("Expected 'default' namespace when kubeconfig doesn't exist, got: %s", namespace)
	}
	
	if err == nil {
		t.Error("Expected error when kubeconfig file doesn't exist, got nil")
	}
}

func TestGetCurrentNamespaceOrProvided_WithProvidedNamespace(t *testing.T) {
	providedNamespace := "provided-namespace"
	
	result := GetCurrentNamespaceOrProvided(&providedNamespace)
	
	if result != providedNamespace {
		t.Errorf("Expected provided namespace %s, got: %s", providedNamespace, result)
	}
}

func TestGetCurrentNamespaceOrProvided_WithoutProvidedNamespace(t *testing.T) {
	// Save original environment variable
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set test environment variable
	testNamespace := "test-namespace-for-default"
	os.Setenv("POD_NAMESPACE", testNamespace)

	result := GetCurrentNamespaceOrProvided(nil)
	
	if result != testNamespace {
		t.Errorf("Expected namespace from environment %s, got: %s", testNamespace, result)
	}
}

func TestGetCurrentNamespaceOrProvided_WithNilPointer(t *testing.T) {
	// Save original environment variable
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		os.Setenv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set test environment variable
	testNamespace := "test-namespace-for-nil-pointer"
	os.Setenv("POD_NAMESPACE", testNamespace)

	var nilPtr *string = nil
	result := GetCurrentNamespaceOrProvided(nilPtr)
	
	if result != testNamespace {
		t.Errorf("Expected namespace from environment %s, got: %s", testNamespace, result)
	}
}