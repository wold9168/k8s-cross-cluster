package main

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// GetAllConfigMapsInCurrentNamespace retrieves all ConfigMaps from the current namespace
func GetAllConfigMapsInCurrentNamespace(clientset kubernetes.Interface, namespace *string) (*v1.ConfigMapList, error) {
	// Get the current namespace
	var ns string
	if namespace == nil {
		currentNamespace, err := getCurrentNamespace()
		if err != nil {
			klog.Warningf("Could not determine current namespace, using 'default': %v", err)
			ns = "default"
		} else {
			ns = currentNamespace
		}
	} else {
		ns = *namespace
	}

	// Attempt to list all ConfigMaps from the current namespace
	configMapList, err := clientset.CoreV1().ConfigMaps(ns).List(context.TODO(), metav1.ListOptions{})

	// Handle different types of errors
	if errors.IsNotFound(err) {
		klog.Errorf("ConfigMaps not found in namespace %s\n", ns)
		return configMapList, err
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		// Handle Kubernetes API status errors (like 403, 500, etc.)
		klog.Errorf("Error listing ConfigMaps in namespace %s: %v\n", ns, statusError.ErrStatus.Message)
		return configMapList, err
	} else if err != nil {
		// Other non-nil errors (like network issues, context cancellation, etc.)
		klog.Errorf("Unexpected error listing ConfigMaps in namespace %s: %v\n", ns, err)
		return configMapList, err
	} else {
		// Success case - ConfigMaps were listed
		klog.Infof("Found %d ConfigMap(s) in namespace %s\n", len(configMapList.Items), ns)
		return configMapList, nil
	}
}