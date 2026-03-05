package generator

import (
	"context"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// ConfigManager manages the complete Caddy configuration lifecycle
type ConfigManager struct {
	serviceDiscovery *ServiceDiscovery
	configGenerator  *CaddyConfigGenerator
	configPath       string
	configDir        string
}

// ConfigManagerOption configures a ConfigManager instance
type ConfigManagerOption func(*ConfigManager)

// WithConfigPath sets the configuration file path
func WithConfigPath(path string) ConfigManagerOption {
	return func(cm *ConfigManager) {
		cm.configPath = path
	}
}

// WithConfigDir sets the configuration directory
func WithConfigDir(dir string) ConfigManagerOption {
	return func(cm *ConfigManager) {
		cm.configDir = dir
	}
}

// NewConfigManager creates a new ConfigManager
func NewConfigManager(clientset kubernetes.Interface, opts ...ConfigManagerOption) *ConfigManager {
	cm := &ConfigManager{
		serviceDiscovery: NewServiceDiscovery(clientset),
		configGenerator:  NewCaddyConfigGenerator(),
		configPath:       "/config/Caddyfile",
		configDir:        "/config",
	}

	for _, opt := range opts {
		opt(cm)
	}

	return cm
}

// LoadClusterName loads cluster name from ConfigMap
func (cm *ConfigManager) LoadClusterName(configMapName string) error {
	return cm.serviceDiscovery.LoadClusterNameFromConfigMap(configMapName)
}

// GenerateConfig generates Caddy configuration from Kubernetes services
func (cm *ConfigManager) GenerateConfig(ctx context.Context) (string, error) {
	// List services
	serviceList, err := cm.serviceDiscovery.ListServices(ctx)
	if err != nil {
		return "", err
	}

	klog.Infof("Retrieved %d services", len(serviceList.Items))
	for _, svc := range serviceList.Items {
		klog.Infof("  - Service: %s/%s", svc.Namespace, svc.Name)
	}

	// Generate domain mappings
	domainResult := cm.serviceDiscovery.GenerateDomainMapping(serviceList)

	// Generate Caddy config
	config := cm.configGenerator.GenerateFromResult(domainResult)

	return config, nil
}

// WriteConfig writes configuration to file
func (cm *ConfigManager) WriteConfig(config string) error {
	// Ensure config directory exists
	if err := os.MkdirAll(cm.configDir, 0755); err != nil {
		klog.Errorf("Failed to create config directory '%s': %v", cm.configDir, err)
		return err
	}

	// Write config file
	if err := os.WriteFile(cm.configPath, []byte(config), 0644); err != nil {
		klog.Errorf("Failed to write Caddy config file '%s': %v", cm.configPath, err)
		return err
	}

	klog.Infof("Successfully wrote Caddy config to '%s'", cm.configPath)
	return nil
}

// Sync performs a complete sync: generate and write config
func (cm *ConfigManager) Sync(ctx context.Context) error {
	config, err := cm.GenerateConfig(ctx)
	if err != nil {
		return err
	}

	return cm.WriteConfig(config)
}

// GetClusterName returns the current cluster name
func (cm *ConfigManager) GetClusterName() string {
	return cm.serviceDiscovery.GetClusterName()
}
