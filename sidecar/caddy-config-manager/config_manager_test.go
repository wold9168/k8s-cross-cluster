package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestConfigManager_New(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cm := NewConfigManager(clientset)

	assert.NotNil(t, cm)
	assert.NotNil(t, cm.serviceDiscovery)
	assert.NotNil(t, cm.configGenerator)
	assert.Equal(t, "/config/Caddyfile", cm.configPath)
	assert.Equal(t, "/config", cm.configDir)
}

func TestConfigManager_WithOptions(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cm := NewConfigManager(
		clientset,
		WithConfigPath("/custom/path/Caddyfile"),
		WithConfigDir("/custom/path"),
	)

	assert.Equal(t, "/custom/path/Caddyfile", cm.configPath)
	assert.Equal(t, "/custom/path", cm.configDir)
}

func TestConfigManager_LoadClusterName(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tailscale-cluster-name",
				Namespace: "default",
			},
			Data: map[string]string{
				"CLUSTER_NAME": "test-cluster",
			},
		},
	)

	cm := NewConfigManager(clientset)
	err := cm.LoadClusterName("tailscale-cluster-name")

	assert.NoError(t, err)
	assert.Equal(t, "test-cluster", cm.GetClusterName())
}

func TestConfigManager_GenerateConfig(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset(
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-svc",
				Namespace: "default",
			},
		},
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tailscale-cluster-name",
				Namespace: "default",
			},
			Data: map[string]string{
				"CLUSTER_NAME": "test-cluster",
			},
		},
	)

	cm := NewConfigManager(clientset)
	err := cm.LoadClusterName("tailscale-cluster-name")
	assert.NoError(t, err)

	config, err := cm.GenerateConfig(ctx)

	assert.NoError(t, err)
	assert.Contains(t, config, "test-svc.default.svc.test-cluster.remote")
	assert.Contains(t, config, "reverse_proxy test-svc.default.svc.cluster.local")
}

func TestConfigManager_GenerateConfig_NoServices(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()

	cm := NewConfigManager(clientset)
	config, err := cm.GenerateConfig(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "", config) // Empty config when no services
}

func TestConfigManager_WriteConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "Caddyfile")

	clientset := fake.NewSimpleClientset()
	cm := NewConfigManager(
		clientset,
		WithConfigPath(configPath),
		WithConfigDir(tmpDir),
	)

	config := `test.remote {
    tls internal
    reverse_proxy test.local
}
`

	err := cm.WriteConfig(config)

	assert.NoError(t, err)

	// Verify file was created
	content, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	assert.Equal(t, config, string(content))
}

func TestConfigManager_WriteConfig_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "Caddyfile")

	clientset := fake.NewSimpleClientset()
	cm := NewConfigManager(
		clientset,
		WithConfigPath(configPath),
		WithConfigDir(filepath.Dir(configPath)),
	)

	config := "test config"
	err := cm.WriteConfig(config)

	assert.NoError(t, err)

	// Verify file was created
	content, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	assert.Equal(t, config, string(content))
}

func TestConfigManager_Sync(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset(
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "sync-svc",
				Namespace: "default",
			},
		},
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tailscale-cluster-name",
				Namespace: "default",
			},
			Data: map[string]string{
				"CLUSTER_NAME": "sync-cluster",
			},
		},
	)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "Caddyfile")

	cm := NewConfigManager(
		clientset,
		WithConfigPath(configPath),
		WithConfigDir(tmpDir),
	)

	// Load cluster name from ConfigMap
	err := cm.LoadClusterName("tailscale-cluster-name")
	assert.NoError(t, err)

	err = cm.Sync(ctx)

	assert.NoError(t, err)

	// Verify config was written
	content, err := os.ReadFile(configPath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "sync-svc.default.svc.sync-cluster.remote")
}

func TestConfigManager_GetClusterName(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cm := NewConfigManager(clientset)
	cm.serviceDiscovery.clusterName = "my-cluster"

	assert.Equal(t, "my-cluster", cm.GetClusterName())
}

func TestConfigManager_ListServices(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset(
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc1",
				Namespace: "default",
			},
		},
	)

	cm := NewConfigManager(clientset)
	services, err := cm.ListServices(ctx)

	assert.NoError(t, err)
	assert.Len(t, services.Items, 1)
}

func TestConfigManager_GenerateDomainMapping(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cm := NewConfigManager(clientset)
	cm.serviceDiscovery.clusterName = "test-cluster"

	serviceList := &v1.ServiceList{
		Items: []v1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mapping-svc",
					Namespace: "test-ns",
				},
			},
		},
	}

	result := cm.GenerateDomainMapping(serviceList)

	assert.Len(t, result.RemoteDomains, 1)
	assert.Contains(t, result.RemoteDomains, "mapping-svc.test-ns.svc.test-cluster.remote")
}

func TestConfigManager_Generate(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	cm := NewConfigManager(clientset)

	remoteDomains := []string{"remote.com"}
	domainMapping := map[string]string{"remote.com": "local.com"}

	config := cm.Generate(remoteDomains, domainMapping)

	assert.Contains(t, config, "remote.com")
	assert.Contains(t, config, "local.com")
}

func TestConfigManagerOption(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	// Test WithConfigPath option
	cm := NewConfigManager(clientset, WithConfigPath("/test/path"))
	assert.Equal(t, "/test/path", cm.configPath)

	// Test WithConfigDir option
	cm2 := NewConfigManager(clientset, WithConfigDir("/test/dir"))
	assert.Equal(t, "/test/dir", cm2.configDir)
}
