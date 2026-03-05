package main

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// GenerateCrossClusterServiceDomains generates cross-cluster service domains
// This is a legacy function kept for backward compatibility
// Deprecated: Use ServiceDiscovery.GenerateDomainMapping instead
func GenerateCrossClusterServiceDomains(clientset kubernetes.Interface, serviceList *v1.ServiceList) ([]string, map[string]string) {
	sd := NewServiceDiscovery(clientset)
	sd.LoadClusterNameFromConfigMap("tailscale-cluster-name")
	result := sd.GenerateDomainMapping(serviceList)
	return result.RemoteDomains, result.DomainMapping
}
