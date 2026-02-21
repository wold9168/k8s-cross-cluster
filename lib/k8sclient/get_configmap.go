package k8sclient

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetAllConfigMapsInCurrentNamespace retrieves all ConfigMaps from the current namespace
func GetAllConfigMapsInCurrentNamespace(clientset kubernetes.Interface, namespace *string) (*v1.ConfigMapList, error) {
	// Get the current namespace
	var ns string
	if namespace == nil {
		currentNamespace, err := GetCurrentNamespace()
		if err != nil {
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
		return configMapList, err
	} else if _, isStatus := err.(*errors.StatusError); isStatus {
		// Handle Kubernetes API status errors (like 403, 500, etc.)
		return configMapList, err
	} else if err != nil {
		// Other non-nil errors (like network issues, context cancellation, etc.)
		return configMapList, err
	} else {
		// Success case - ConfigMaps were listed
		return configMapList, nil
	}
}

// CreateConfigMap 创建指定命名空间下的 ConfigMap
func CreateConfigMap(clientset kubernetes.Interface, namespace *string, cm *v1.ConfigMap) (*v1.ConfigMap, error) {
	if clientset == nil || cm == nil {
		return nil, errors.NewBadRequest("clientset or configmap is nil")
	}

	ns := GetCurrentNamespaceOrProvided(namespace)

	createdCm, err := clientset.CoreV1().ConfigMaps(ns).Create(context.TODO(), cm, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdCm, nil
}

// GetConfigMap 获取指定命名空间下的 ConfigMap
func GetConfigMap(clientset kubernetes.Interface, namespace *string, name string) (*v1.ConfigMap, error) {
	if clientset == nil {
		return nil, errors.NewBadRequest("clientset is nil")
	}

	ns := GetCurrentNamespaceOrProvided(namespace)

	cm, err := clientset.CoreV1().ConfigMaps(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return cm, nil
}

// UpdateExistingConfigMap 更新指定的 ConfigMap
func UpdateExistingConfigMap(clientset kubernetes.Interface, namespace *string, cm *v1.ConfigMap) (*v1.ConfigMap, error) {
	if clientset == nil || cm == nil {
		return nil, errors.NewBadRequest("clientset or configmap is nil")
	}

	ns := GetCurrentNamespaceOrProvided(namespace)

	updatedCm, err := clientset.CoreV1().ConfigMaps(ns).Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return updatedCm, nil
}

// DeleteConfigMap 删除指定命名空间下的 ConfigMap
func DeleteConfigMap(clientset kubernetes.Interface, namespace *string, name string) error {
	if clientset == nil {
		return errors.NewBadRequest("clientset is nil")
	}

	ns := GetCurrentNamespaceOrProvided(namespace)

	err := clientset.CoreV1().ConfigMaps(ns).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	return nil
}
