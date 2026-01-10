package main

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// GetAllServicesInCurrentNamespace retrieves all Services from the current namespace
func GetAllServicesInCurrentNamespace(clientset kubernetes.Interface, namespace *string) (*v1.ServiceList, error) {
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

	// Attempt to list all Services from the current namespace
	serviceList, err := clientset.CoreV1().Services(ns).List(context.TODO(), metav1.ListOptions{})

	// Handle different types of errors
	if errors.IsNotFound(err) {
		klog.Errorf("Services not found in namespace %s\n", ns)
		return serviceList, err
	} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
		// Handle Kubernetes API status errors (like 403, 500, etc.)
		klog.Errorf("Error listing Services in namespace %s: %v\n", ns, statusError.ErrStatus.Message)
		return serviceList, err
	} else if err != nil {
		// Other non-nil errors (like network issues, context cancellation, etc.)
		klog.Errorf("Unexpected error listing Services in namespace %s: %v\n", ns, err)
		return serviceList, err
	} else {
		// Success case - Services were listed
		klog.Infof("Found %d Service(s) in namespace %s\n", len(serviceList.Items), ns)
		return serviceList, nil
	}
}