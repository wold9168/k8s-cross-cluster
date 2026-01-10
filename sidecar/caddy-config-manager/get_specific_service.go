package main

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// GetSpecificServiceInCurrentNamespace retrieves a specific Service from the current namespace
func GetSpecificServiceInCurrentNamespace(clientset *kubernetes.Clientset, serviceName string) (*v1.Service, error) {
	// Get the current namespace
	namespace, err := getCurrentNamespace()
	if err != nil {
		klog.Warningf("Could not determine current namespace, using 'default': %v", err)
		namespace = "default"
	}

	// Attempt to get the Service from the current namespace
	service, err := clientset.CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})

	// Handle different types of errors
	if errors.IsNotFound(err) {
		klog.Errorf("Service %s not found in namespace %s\n", serviceName, namespace)
		return service, err
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		// Handle Kubernetes API status errors (like 403, 500, etc.)
		klog.Errorf("Error getting Service %s in namespace %s: %v\n", serviceName, namespace, statusError.ErrStatus.Message)
		return service, err
	} else if err != nil {
		// Other non-nil errors (like network issues, context cancellation, etc.)
		klog.Errorf("Unexpected error getting Service %s in namespace %s: %v\n", serviceName, namespace, err)
		return service, err
	} else {
		// Success case - Service was found
		klog.Infof("Found Service %s in namespace %s\n", serviceName, namespace)
		return service, nil
	}
}