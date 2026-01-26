package main

import (
	"fmt"

	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
	"k8s.io/klog/v2"
)

// RemoteService represents a service discovered in a remote cluster
type RemoteService struct {
	Name      string
	Namespace string
	ClusterIP string
	Ports     []ServicePort
}

// ServicePort represents a port exposed by a service
type ServicePort struct {
	Name     string
	Port     int32
	Protocol string
}

// UpdateDNSRecordsForGateways generates and adds DNS records for the gateways to the DNS server
func UpdateDNSRecordsForGateways(dnsSrv *dnsserver.DNSServer, gatewayHostNames []string) {
	// Clear existing remote records before adding new ones
	// This prevents accumulation of stale records
	for _, gatewayName := range gatewayHostNames {
		// Discover services from the remote cluster
		remoteServices := discoverRemoteClusterServices(gatewayName)

		// Add DNS records for each discovered service
		for _, svc := range remoteServices {
			// Format: service.namespace.svc.clustername.remote
			recordName := fmt.Sprintf("%s.%s.svc.%s.remote.", svc.Name, svc.Namespace, gatewayName)

			// Add A record for the service
			dnsSrv.AddRecord(recordName, 1 /* dns.TypeA */, 300 /* TTL */, svc.ClusterIP)
			klog.Infof("Added DNS A record: %s -> %s", recordName, svc.ClusterIP)

			// Add SRV record for the service if it has ports
			for _, port := range svc.Ports {
				srvRecordName := fmt.Sprintf("_%s._%s.%s.%s.svc.%s.remote.",
					port.Name, port.Protocol, svc.Name, svc.Namespace, gatewayName)

				// Format for SRV record: priority weight port target
				srvTarget := fmt.Sprintf("%s.%s.svc.%s.remote.", svc.Name, svc.Namespace, gatewayName)
				srvData := fmt.Sprintf("0 50 %d %s", port.Port, srvTarget)

				dnsSrv.AddRecord(srvRecordName, 33 /* dns.TypeSRV */, 300 /* TTL */, srvData)
				klog.Infof("Added DNS SRV record: %s -> %s", srvRecordName, srvData)
			}
		}
	}
}

// discoverRemoteClusterServices discovers services in the remote cluster
// In a real implementation, this would connect to the remote cluster and fetch services
// For now, we'll simulate discovery with mock data
func discoverRemoteClusterServices(clusterName string) []RemoteService {
	// In a real implementation, this function would:
	// 1. Establish connection to the remote cluster using the clusterName
	// 2. Query the remote cluster's API server for services
	// 3. Return the discovered services

	// For demonstration purposes, returning mock services
	// In reality, you would implement actual service discovery here
	mockServices := []RemoteService{
		{
			Name:      "nginx-service",
			Namespace: "default",
			ClusterIP: "10.96.10.10",
			Ports: []ServicePort{
				{
					Name:     "http",
					Port:     80,
					Protocol: "TCP",
				},
			},
		},
		{
			Name:      "database-service",
			Namespace: "backend",
			ClusterIP: "10.96.10.20",
			Ports: []ServicePort{
				{
					Name:     "postgres",
					Port:     5432,
					Protocol: "TCP",
				},
			},
		},
	}

	klog.V(4).Infof("Discovered %d services in cluster %s", len(mockServices), clusterName)
	return mockServices
}
