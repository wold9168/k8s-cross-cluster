package main

import (
	"context"
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetAllConfigMapsInCurrentNamespace(t *testing.T) {
	// 创建fake clientset
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap1",
				Namespace: namespace,
			},
		},
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmap2",
				Namespace: namespace,
			},
		},
	)

	// 调用函数，传入namespace参数
	configMapList, err := GetAllConfigMapsInCurrentNamespace(clientset, &namespace)

	// 验证结果
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if configMapList == nil {
		t.Fatal("Expected non-nil ConfigMapList")
	}

	if len(configMapList.Items) != 2 {
		t.Errorf("Expected 2 ConfigMaps, got: %d", len(configMapList.Items))
	}

	// 验证ConfigMap名称
	names := []string{configMapList.Items[0].Name, configMapList.Items[1].Name}
	if names[0] != "configmap1" && names[1] != "configmap1" {
		t.Errorf("Expected configmap1 in the list, got: %v", names)
	}
	if names[0] != "configmap2" && names[1] != "configmap2" {
		t.Errorf("Expected configmap2 in the list, got: %v", names)
	}
}

func TestGetAllConfigMapsInCurrentNamespace_Empty(t *testing.T) {
	// 创建空的fake clientset
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset()

	// 调用函数，传入namespace参数
	configMapList, err := GetAllConfigMapsInCurrentNamespace(clientset, &namespace)

	// 验证结果
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if configMapList == nil {
		t.Fatal("Expected non-nil ConfigMapList")
	}

	if len(configMapList.Items) != 0 {
		t.Errorf("Expected 0 ConfigMaps, got: %d", len(configMapList.Items))
	}
}

func TestGetAllServicesInCurrentNamespace(t *testing.T) {
	// 创建fake clientset
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset(
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service1",
				Namespace: namespace,
			},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "service2",
				Namespace: namespace,
			},
		},
	)

	// 调用函数，传入namespace参数
	serviceList, err := GetAllServicesInCurrentNamespace(clientset, &namespace)

	// 验证结果
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if serviceList == nil {
		t.Fatal("Expected non-nil ServiceList")
	}

	if len(serviceList.Items) != 2 {
		t.Errorf("Expected 2 Services, got: %d", len(serviceList.Items))
	}

	// 验证Service名称
	names := []string{serviceList.Items[0].Name, serviceList.Items[1].Name}
	if names[0] != "service1" && names[1] != "service1" {
		t.Errorf("Expected service1 in the list, got: %v", names)
	}
	if names[0] != "service2" && names[1] != "service2" {
		t.Errorf("Expected service2 in the list, got: %v", names)
	}
}

func TestGetAllServicesInCurrentNamespace_Empty(t *testing.T) {
	// 创建空的fake clientset
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset()

	// 调用函数，传入namespace参数
	serviceList, err := GetAllServicesInCurrentNamespace(clientset, &namespace)

	// 验证结果
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if serviceList == nil {
		t.Fatal("Expected non-nil ServiceList")
	}

	if len(serviceList.Items) != 0 {
		t.Errorf("Expected 0 Services, got: %d", len(serviceList.Items))
	}
}

func TestGenerateCrossClusterServiceDomains(t *testing.T) {
	namespace := "test-ns"
	serviceList := &v1.ServiceList{
		Items: []v1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: namespace,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service2",
					Namespace: namespace,
				},
			},
		},
	}

	remoteDomains, domainMapping := GenerateCrossClusterServiceDomains(serviceList)

	if len(remoteDomains) != 2 {
		t.Errorf("Expected 2 remote domains, got: %d", len(remoteDomains))
	}

	if len(domainMapping) != 2 {
		t.Errorf("Expected 2 domain mappings, got: %d", len(domainMapping))
	}

	expectedRemote1 := "service1.test-ns.svc.clusterwise.remote"
	expectedLocal1 := "service1.test-ns.svc.cluster.local"
	expectedRemote2 := "service2.test-ns.svc.clusterwise.remote"
	expectedLocal2 := "service2.test-ns.svc.cluster.local"

	foundRemote1 := false
	foundRemote2 := false

	for _, domain := range remoteDomains {
		if domain == expectedRemote1 {
			foundRemote1 = true
		}
		if domain == expectedRemote2 {
			foundRemote2 = true
		}
	}

	if !foundRemote1 {
		t.Errorf("Expected to find remote domain: %s", expectedRemote1)
	}
	if !foundRemote2 {
		t.Errorf("Expected to find remote domain: %s", expectedRemote2)
	}

	if domainMapping[expectedRemote1] != expectedLocal1 {
		t.Errorf("Expected mapping %s -> %s, got: %s", expectedRemote1, expectedLocal1, domainMapping[expectedRemote1])
	}
	if domainMapping[expectedRemote2] != expectedLocal2 {
		t.Errorf("Expected mapping %s -> %s, got: %s", expectedRemote2, expectedLocal2, domainMapping[expectedRemote2])
	}
}

func TestGenerateCrossClusterServiceDomains_Nil(t *testing.T) {
	remoteDomains, domainMapping := GenerateCrossClusterServiceDomains(nil)

	if len(remoteDomains) != 0 {
		t.Errorf("Expected 0 remote domains, got: %d", len(remoteDomains))
	}

	if len(domainMapping) != 0 {
		t.Errorf("Expected 0 domain mappings, got: %d", len(domainMapping))
	}
}

func TestGenerateCrossClusterServiceDomains_Empty(t *testing.T) {
	serviceList := &v1.ServiceList{
		Items: []v1.Service{},
	}

	remoteDomains, domainMapping := GenerateCrossClusterServiceDomains(serviceList)

	if len(remoteDomains) != 0 {
		t.Errorf("Expected 0 remote domains, got: %d", len(remoteDomains))
	}

	if len(domainMapping) != 0 {
		t.Errorf("Expected 0 domain mappings, got: %d", len(domainMapping))
	}
}

func TestGenerateCaddyConfig(t *testing.T) {
	remoteDomains := []string{
		"service1.test-ns.svc.clusterwise.remote",
		"service2.test-ns.svc.clusterwise.remote",
	}
	domainMapping := map[string]string{
		"service1.test-ns.svc.clusterwise.remote": "service1.test-ns.svc.cluster.local",
		"service2.test-ns.svc.clusterwise.remote": "service2.test-ns.svc.cluster.local",
	}

	config := GenerateCaddyConfig(remoteDomains, domainMapping)

	expected := `service1.test-ns.svc.clusterwise.remote {
    reverse_proxy service1.test-ns.svc.cluster.local
}
service2.test-ns.svc.clusterwise.remote {
    reverse_proxy service2.test-ns.svc.cluster.local
}
`

	if config != expected {
		t.Errorf("Expected config:\n%s\nGot:\n%s", expected, config)
	}
}

func TestGenerateCaddyConfig_Empty(t *testing.T) {
	remoteDomains := []string{}
	domainMapping := map[string]string{}

	config := GenerateCaddyConfig(remoteDomains, domainMapping)

	if config != "" {
		t.Errorf("Expected empty config, got: %s", config)
	}
}

func TestGenerateCaddyConfig_MissingMapping(t *testing.T) {
	remoteDomains := []string{
		"service1.test-ns.svc.clusterwise.remote",
	}
	domainMapping := map[string]string{} // 空映射

	config := GenerateCaddyConfig(remoteDomains, domainMapping)

	if config != "" {
		t.Errorf("Expected empty config due to missing mapping, got: %s", config)
	}
}

func TestUpdateCaddyConfigMap_Create(t *testing.T) {
	namespace := "test-ns"
	clientset := fake.NewSimpleClientset()
	caddyConfig := "service1.test-ns.svc.clusterwise.remote {\n    reverse_proxy service1.test-ns.svc.cluster.local\n}\n"

	err := UpdateCaddyConfigMap(clientset, &namespace, caddyConfig)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// 验证 ConfigMap 是否已创建
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), caddyConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get ConfigMap: %v", err)
	}

	if cm.Data[caddyConfigKey] != caddyConfig {
		t.Errorf("Expected Caddyfile content: %s, got: %s", caddyConfig, cm.Data[caddyConfigKey])
	}
}

func TestUpdateCaddyConfigMap_Update(t *testing.T) {
	namespace := "test-ns"
	existingConfig := "old config"
	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      caddyConfigMapName,
				Namespace: namespace,
			},
			Data: map[string]string{
				caddyConfigKey: existingConfig,
			},
		},
	)
	newConfig := "service1.test-ns.svc.clusterwise.remote {\n    reverse_proxy service1.test-ns.svc.cluster.local\n}\n"

	err := UpdateCaddyConfigMap(clientset, &namespace, newConfig)

	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// 验证 ConfigMap 是否已更新
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.Background(), caddyConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Errorf("Failed to get ConfigMap: %v", err)
	}

	if cm.Data[caddyConfigKey] != newConfig {
		t.Errorf("Expected Caddyfile content: %s, got: %s", newConfig, cm.Data[caddyConfigKey])
	}

	if cm.Data[caddyConfigKey] == existingConfig {
		t.Errorf("ConfigMap was not updated, still has old config")
	}
}