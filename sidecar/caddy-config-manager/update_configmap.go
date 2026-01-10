package main

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const caddyConfigMapName = "caddy-config"
const caddyConfigKey = "Caddyfile"

// UpdateCaddyConfigMap creates or updates the ConfigMap with Caddy configuration
func getCurrentNamespaceOrProvided(namespace *string) string {
	if namespace != nil {
		return *namespace
	}
	
	currentNamespace, err := getCurrentNamespace()
	if err != nil {
		klog.Warningf("Could not determine current namespace, using 'default': %v", err)
		return "default"
	}
	return currentNamespace
}

// UpdateCaddyConfigMap creates or updates the ConfigMap with Caddy configuration
func UpdateCaddyConfigMap(clientset kubernetes.Interface, namespaceProvided *string, caddyConfig string) error {
	ctx := context.Background()
	ns := getCurrentNamespaceOrProvided(namespaceProvided)

	configMaps := clientset.CoreV1().ConfigMaps(ns)

	// Check if ConfigMap exists
	existingCM, err := configMaps.Get(ctx, caddyConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap does not exist, create it
			newCM := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      caddyConfigMapName,
					Namespace: ns,
				},
				Data: map[string]string{
					caddyConfigKey: caddyConfig,
				},
			}
			_, err = configMaps.Create(ctx, newCM, metav1.CreateOptions{})
			if err != nil {
				klog.Errorf("Failed to create ConfigMap %s: %v", caddyConfigMapName, err)
				return err
			}
			klog.Infof("Created ConfigMap %s successfully", caddyConfigMapName)
			return nil
		}
		klog.Errorf("Failed to get ConfigMap %s: %v", caddyConfigMapName, err)
		return err
	}

	// ConfigMap exists, update it
	if existingCM.Data == nil {
		existingCM.Data = make(map[string]string)
	}
	existingCM.Data[caddyConfigKey] = caddyConfig

	_, err = configMaps.Update(ctx, existingCM, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update ConfigMap %s: %v", caddyConfigMapName, err)
		return err
	}
	klog.Infof("Updated ConfigMap %s successfully", caddyConfigMapName)
	return nil
}