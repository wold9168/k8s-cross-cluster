package k8sclient

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const CaddyConfigMapName = "caddy-config"
const CaddyConfigKey = "Caddyfile"

// UpdateCaddyConfigMap creates or updates the ConfigMap with Caddy configuration
func UpdateCaddyConfigMap(clientset kubernetes.Interface, namespaceProvided *string, caddyConfig string) error {
	ctx := context.Background()
	ns := getCurrentNamespaceOrProvided(namespaceProvided)

	configMaps := clientset.CoreV1().ConfigMaps(ns)

	// Check if ConfigMap exists
	existingCM, err := configMaps.Get(ctx, CaddyConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap does not exist, create it
			newCM := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      CaddyConfigMapName,
					Namespace: ns,
				},
				Data: map[string]string{
					CaddyConfigKey: caddyConfig,
				},
			}
			_, err = configMaps.Create(ctx, newCM, metav1.CreateOptions{})
			if err != nil {
				klog.Errorf("Failed to create ConfigMap %s: %v", CaddyConfigMapName, err)
				return err
			}
			klog.Infof("Created ConfigMap %s successfully", CaddyConfigMapName)
			return nil
		}
		klog.Errorf("Failed to get ConfigMap %s: %v", CaddyConfigMapName, err)
		return err
	}

	// ConfigMap exists, update it
	if existingCM.Data == nil {
		existingCM.Data = make(map[string]string)
	}
	existingCM.Data[CaddyConfigKey] = caddyConfig

	_, err = configMaps.Update(ctx, existingCM, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update ConfigMap %s: %v", CaddyConfigMapName, err)
		return err
	}
	klog.Infof("Updated ConfigMap %s successfully", CaddyConfigMapName)
	return nil
}