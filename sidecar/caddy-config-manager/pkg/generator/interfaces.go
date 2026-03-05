package generator

import (
	"context"

	v1 "k8s.io/api/core/v1"
)

// ServiceLister defines the interface for listing Kubernetes services
type ServiceLister interface {
	ListServices(ctx context.Context) (*v1.ServiceList, error)
}

// DomainGenerator defines the interface for generating domain mappings
type DomainGenerator interface {
	GenerateDomainMapping(serviceList *v1.ServiceList) DomainMappingResult
}

// ConfigGenerator defines the interface for generating Caddy configuration
type ConfigGenerator interface {
	Generate(remoteDomains []string, domainMapping map[string]string) string
}

// ConfigWriter defines the interface for writing configuration to storage
type ConfigWriter interface {
	WriteConfig(config string) error
}

// ClusterNameLoader defines the interface for loading cluster name
type ClusterNameLoader interface {
	LoadClusterName(configMapName string) error
	GetClusterName() string
}

// ServiceDiscovery implements multiple interfaces
var (
	_ ServiceLister   = (*ServiceDiscovery)(nil)
	_ DomainGenerator = (*ServiceDiscovery)(nil)
)

// CaddyConfigGenerator implements ConfigGenerator
var (
	_ ConfigGenerator = (*CaddyConfigGenerator)(nil)
)

// ConfigManager implements the complete lifecycle interfaces
var (
	_ ServiceLister   = (*ConfigManager)(nil)
	_ DomainGenerator = (*ConfigManager)(nil)
	_ ConfigGenerator = (*ConfigManager)(nil)
	_ ConfigWriter    = (*ConfigManager)(nil)
)

// ListServices delegates to serviceDiscovery
func (cm *ConfigManager) ListServices(ctx context.Context) (*v1.ServiceList, error) {
	return cm.serviceDiscovery.ListServices(ctx)
}

// GenerateDomainMapping delegates to serviceDiscovery
func (cm *ConfigManager) GenerateDomainMapping(serviceList *v1.ServiceList) DomainMappingResult {
	return cm.serviceDiscovery.GenerateDomainMapping(serviceList)
}

// Generate generates Caddy config using the internal generator
func (cm *ConfigManager) Generate(remoteDomains []string, domainMapping map[string]string) string {
	return cm.configGenerator.Generate(remoteDomains, domainMapping)
}
