package k8sclient

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
)

const (
	CaddyConfigMapName = "caddy-config"
	CaddyConfigKey     = "Caddyfile"
)

// updateExistingConfigMap updates an existing ConfigMap with new data
func updateExistingConfigMap(ctx context.Context, configMaps corev1.ConfigMapInterface, existingCM *v1.ConfigMap, configMapName string, key string, data string) error {
	if existingCM.Data == nil {
		existingCM.Data = make(map[string]string)
	}
	existingCM.Data[key] = data

	_, err := configMaps.Update(ctx, existingCM, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update ConfigMap %s: %v", configMapName, err)
		return err
	}
	klog.Infof("Updated ConfigMap %s successfully", configMapName)
	return nil
}

// createConfigMap creates a new ConfigMap with the specified data
func createConfigMap(ctx context.Context, configMaps corev1.ConfigMapInterface, ns string, configMapName string, key string, data string) error {
	newCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: ns,
		},
		Data: map[string]string{
			key: data,
		},
	}
	_, err := configMaps.Create(ctx, newCM, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("Failed to create ConfigMap %s: %v", configMapName, err)
		return err
	}
	klog.Infof("Created ConfigMap %s successfully", configMapName)
	return nil
}

// UpdateConfigMap creates or updates a ConfigMap with the specified data
func UpdateConfigMap(clientset kubernetes.Interface, namespaceProvided *string, configMapName string, key string, data string) error {
	ctx := context.Background()
	ns := GetCurrentNamespaceOrProvided(namespaceProvided)

	configMaps := clientset.CoreV1().ConfigMaps(ns)

	existingCM, err := configMaps.Get(ctx, configMapName, metav1.GetOptions{})
	if err == nil {
		return updateExistingConfigMap(ctx, configMaps, existingCM, configMapName, key, data)
	} else if errors.IsNotFound(err) {
		return createConfigMap(ctx, configMaps, ns, configMapName, key, data)
	} else {
		klog.Errorf("Failed to get ConfigMap %s: %v", configMapName, err)
		return err
	}
}

// UpdateCaddyConfigMap creates or updates the ConfigMap with Caddy configuration
func UpdateCaddyConfigMap(clientset kubernetes.Interface, namespaceProvided *string, caddyConfig string) error {
	return UpdateConfigMap(clientset, namespaceProvided, CaddyConfigMapName, CaddyConfigKey, caddyConfig)
}
