package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
)

// ServiceDiscovery discovers Kubernetes services and generates cross-cluster domains
type ServiceDiscovery struct {
	clientset   kubernetes.Interface
	clusterName string
	namespace   string
}

// ServiceDiscoveryOption configures a ServiceDiscovery instance
type ServiceDiscoveryOption func(*ServiceDiscovery)

// WithClusterName sets a custom cluster name
func WithClusterName(name string) ServiceDiscoveryOption {
	return func(sd *ServiceDiscovery) {
		sd.clusterName = name
	}
}

// WithNamespace sets the namespace to query
func WithNamespace(ns string) ServiceDiscoveryOption {
	return func(sd *ServiceDiscovery) {
		sd.namespace = ns
	}
}

// NewServiceDiscovery creates a new ServiceDiscovery instance
func NewServiceDiscovery(clientset kubernetes.Interface, opts ...ServiceDiscoveryOption) *ServiceDiscovery {
	sd := &ServiceDiscovery{
		clientset:   clientset,
		clusterName: "default-cluster-name",
		namespace:   "default",
	}

	for _, opt := range opts {
		opt(sd)
	}

	return sd
}

// LoadClusterNameFromConfigMap loads the cluster name from a ConfigMap
func (sd *ServiceDiscovery) LoadClusterNameFromConfigMap(configMapName string) error {
	configMap, err := k8sclient.GetConfigMap(sd.clientset, &sd.namespace, configMapName)
	if err != nil {
		klog.Warningf("Failed to get %s ConfigMap: %v, using default cluster name '%s'", configMapName, err, sd.clusterName)
		return nil // Don't fail, use default
	}

	if name, exists := configMap.Data["CLUSTER_NAME"]; exists && name != "" {
		sd.clusterName = name
		klog.Infof("Loaded cluster name from ConfigMap: CLUSTER_NAME = %s", sd.clusterName)
	} else {
		klog.Warningf("CLUSTER_NAME not found or empty in ConfigMap, using default '%s'", sd.clusterName)
	}

	return nil
}

// GetClusterName returns the current cluster name
func (sd *ServiceDiscovery) GetClusterName() string {
	return sd.clusterName
}

// ListServices lists all services in the configured namespace
func (sd *ServiceDiscovery) ListServices(ctx context.Context) (*v1.ServiceList, error) {
	return sd.clientset.CoreV1().Services(sd.namespace).List(ctx, metav1.ListOptions{})
}

// GenerateDomainMapping generates cross-cluster domain mappings for services
func (sd *ServiceDiscovery) GenerateDomainMapping(serviceList *v1.ServiceList) DomainMappingResult {
	result := DomainMappingResult{
		RemoteDomains: make([]string, 0),
		DomainMapping: make(map[string]string),
	}

	if serviceList == nil {
		return result
	}

	for _, service := range serviceList.Items {
		mapping := sd.generateServiceDomainMapping(service)
		result.RemoteDomains = append(result.RemoteDomains, mapping.RemoteDomain)
		result.DomainMapping[mapping.RemoteDomain] = mapping.LocalDomain
	}

	klog.Infof("Generated domain mappings for %d services", len(result.RemoteDomains))
	return result
}

// generateServiceDomainMapping generates domain mapping for a single service
func (sd *ServiceDiscovery) generateServiceDomainMapping(service v1.Service) DomainMapping {
	serviceName := service.Name
	namespace := service.Namespace

	// Remote domain: <service>.<namespace>.svc.<cluster>.remote
	remoteDomain := fmt.Sprintf("%s.%s.svc.%s.remote", serviceName, namespace, sd.clusterName)

	// Local domain: <service>.<namespace>.svc.cluster.local
	localDomain := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, namespace)

	return DomainMapping{
		RemoteDomain: remoteDomain,
		LocalDomain:  localDomain,
	}
}
