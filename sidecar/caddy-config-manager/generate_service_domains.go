package main

import (
	v1 "k8s.io/api/core/v1"
)

// GenerateCrossClusterServiceDomains generates cross-cluster access domains for services
// Returns a slice of remote domains and a map from remote domain to local domain
func GenerateCrossClusterServiceDomains(serviceList *v1.ServiceList) ([]string, map[string]string) {
	remoteDomains := make([]string, 0)
	domainMapping := make(map[string]string)

	if serviceList == nil {
		return remoteDomains, domainMapping
	}

	for _, service := range serviceList.Items {
		serviceName := service.Name
		namespace := service.Namespace

		// Generate remote domain format: <service-name>.<namespace>.svc.clusterwise.remote
		remoteDomain := serviceName + "." + namespace + ".svc.clusterwise.remote"

		// Generate local domain format: <service-name>.<namespace>.svc.cluster.local
		localDomain := serviceName + "." + namespace + ".svc.cluster.local"

		// Add to slice
		remoteDomains = append(remoteDomains, remoteDomain)

		// Add to mapping
		domainMapping[remoteDomain] = localDomain
	}

	return remoteDomains, domainMapping
}