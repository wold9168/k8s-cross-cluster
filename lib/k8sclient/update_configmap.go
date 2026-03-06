package k8sclient

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	CaddyConfigMapName = "caddy-config"
	CaddyConfigKey     = "Caddyfile"
)

// updateExistingConfigMap updates an existing ConfigMap with new data
func updateExistingConfigMap(clientset kubernetes.Interface, namespace *string, existingCM *v1.ConfigMap, configMapName string, key string, data string) error {
	if existingCM.Data == nil {
		existingCM.Data = make(map[string]string)
	}
	existingCM.Data[key] = data

	_, err := UpdateExistingConfigMap(clientset, namespace, existingCM)
	if err != nil {
		return err
	}
	return nil
}

// createConfigMap creates a new ConfigMap with the specified data
func createConfigMap(clientset kubernetes.Interface, ns *string, configMapName string, key string, data string) error {
	newCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: *ns,
		},
		Data: map[string]string{
			key: data,
		},
	}
	_, err := CreateConfigMap(clientset, ns, newCM)
	if err != nil {
		return err
	}
	return nil
}

// UpdateConfigMap creates or updates a ConfigMap with the specified data
func UpdateConfigMap(clientset kubernetes.Interface, namespaceProvided *string, configMapName string, key string, data string) error {
	ns := GetCurrentNamespaceOrProvided(namespaceProvided)

	existingCM, err := GetConfigMap(clientset, &ns, configMapName)
	if err == nil {
		return updateExistingConfigMap(clientset, &ns, existingCM, configMapName, key, data)
	} else if errors.IsNotFound(err) {
		return createConfigMap(clientset, &ns, configMapName, key, data)
	} else {
		return err
	}
}

// UpdateCaddyConfigMap creates or updates the ConfigMap with Caddy configuration
func UpdateCaddyConfigMap(clientset kubernetes.Interface, namespaceProvided *string, caddyConfig string) error {
	return UpdateConfigMap(clientset, namespaceProvided, CaddyConfigMapName, CaddyConfigKey, caddyConfig)
}