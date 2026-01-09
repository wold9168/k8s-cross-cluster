package main

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// GetSpecificConfigMapInCurrentNamespace retrieves a specific ConfigMap from the current namespace
func GetSpecificConfigMapInCurrentNamespace(clientset *kubernetes.Clientset, configMapName string) (*v1.ConfigMap, error) {
	// Get the current namespace
	namespace, err := getCurrentNamespace()
	if err != nil {
		klog.Warningf("Could not determine current namespace, using 'default': %v", err)
		namespace = "default"
	}

	// Attempt to get the ConfigMap from the current namespace
	configMap, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, metav1.GetOptions{})

	// Handle different types of errors
	if errors.IsNotFound(err) {
		klog.Errorf("ConfigMap %s not found in namespace %s\n", configMapName, namespace)
		return configMap, err
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		// Handle Kubernetes API status errors (like 403, 500, etc.)
		klog.Errorf("Error getting ConfigMap %s in namespace %s: %v\n", configMapName, namespace, statusError.ErrStatus.Message)
		return configMap, err
	} else if err != nil {
		// Other non-nil errors (like network issues, context cancellation, etc.)
		klog.Errorf("Unexpected error getting ConfigMap %s in namespace %s: %v\n", configMapName, namespace, err)
		return configMap, err
	} else {
		// Success case - ConfigMap was found
		klog.Infof("Found ConfigMap %s in namespace %s\n", configMapName, namespace)
		return configMap, nil
	}
}
