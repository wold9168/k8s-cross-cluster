package k8sclient

import (
	"context"
	"fmt"
	"os"

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
		currentNamespace, err := GetCurrentNamespace()
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

// GetCurrentPodServiceClusterIP retrieves the ClusterIP of the Service that selects the current Pod
// It uses the Pod's labels to find a matching Service in the same namespace
func GetCurrentPodServiceClusterIP(clientset kubernetes.Interface) (string, error) {
	// Get Pod name from environment variable
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		return "", fmt.Errorf("POD_NAME environment variable not set")
	}

	// Get namespace
	namespace, err := GetCurrentNamespace()
	if err != nil {
		return "", fmt.Errorf("failed to get namespace: %w", err)
	}

	// Get Pod object to retrieve its labels
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	podLabels := pod.Labels
	if len(podLabels) == 0 {
		return "", fmt.Errorf("pod %s/%s has no labels", namespace, podName)
	}

	// List all Services in the namespace
	serviceList, err := clientset.CoreV1().Services(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to list services in namespace %s: %w", namespace, err)
	}

	// Find the Service whose selector matches the Pod's labels
	for _, service := range serviceList.Items {
		serviceSelector := service.Spec.Selector
		if len(serviceSelector) == 0 {
			continue
		}

		// Check if all key-value pairs in serviceSelector match podLabels
		matches := true
		for key, value := range serviceSelector {
			if podLabels[key] != value {
				matches = false
				break
			}
		}

		if matches {
			klog.Infof("Found matching Service %s for Pod %s/%s with ClusterIP %s", service.Name, namespace, podName, service.Spec.ClusterIP)
			return service.Spec.ClusterIP, nil
		}
	}

	return "", fmt.Errorf("no Service found that selects pod %s/%s", namespace, podName)
}