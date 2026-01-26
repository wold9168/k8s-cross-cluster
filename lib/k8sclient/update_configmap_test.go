package k8sclient

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestUpdateConfigMap_Create(t *testing.T) {
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset()
	configMapName := "test-configmap"
	key := "test-key"
	data := "test-data"

	err := UpdateConfigMap(clientset, &namespace, configMapName, key, data)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify ConfigMap was created
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get ConfigMap: %v", err)
	}

	if cm.Data[key] != data {
		t.Errorf("Expected data content: %s, got: %s", data, cm.Data[key])
	}
}

func TestUpdateConfigMap_Update(t *testing.T) {
	namespace := "test-ns"
	configMapName := "test-configmap"
	key := "test-key"
	existingData := "existing data"
	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: namespace,
			},
			Data: map[string]string{
				key: existingData,
			},
		},
	)
	newData := "new data"

	err := UpdateConfigMap(clientset, &namespace, configMapName, key, newData)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify ConfigMap was updated
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get ConfigMap: %v", err)
	}

	if cm.Data[key] != newData {
		t.Errorf("Expected data content: %s, got: %s", newData, cm.Data[key])
	}

	if cm.Data[key] == existingData {
		t.Errorf("ConfigMap was not updated, still has old data")
	}
}

func TestUpdateConfigMap_NilData(t *testing.T) {
	namespace := "test-ns"
	configMapName := "test-configmap-with-nil-data"
	key := "test-key"
	data := "test-data"
	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: namespace,
			},
			// Data is nil initially
		},
	)

	err := UpdateConfigMap(clientset, &namespace, configMapName, key, data)

	if err != nil {
		t.Errorf("Expected no error when updating ConfigMap with nil data, got: %v", err)
	}

	// Verify ConfigMap was updated
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), configMapName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get ConfigMap: %v", err)
	}

	if cm.Data[key] != data {
		t.Errorf("Expected data content: %s, got: %s", data, cm.Data[key])
	}
}