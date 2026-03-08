package main

import (
	"net/netip"
)

// PeerInfo represents information about a Tailscale peer node
type PeerInfo struct {
	ID           string
	HostName     string
	DNSName      string
	TailscaleIPs []string
	Online       bool
}

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

// DNSRecord represents a DNS record entry
type DNSRecord struct {
	Name  string
	Type  uint16
	TTL   uint32
	Value string
}

// CoreDNSConfig represents CoreDNS configuration state
type CoreDNSConfig struct {
	Namespace       string
	ConfigMapName   string
	ConfigKey       string
	DeploymentName  string
	ManagedSection  ManagedSection
}

// ManagedSection represents the managed portion of Corefile
type ManagedSection struct {
	StartMarker string
	EndMarker   string
}

// RemoteServiceList represents the service list response from remote cluster's /svc endpoint
type RemoteServiceList struct {
	Timestamp int64                        `json:"timestamp"`
	Services  map[string][]RemoteService   `json:"services"`
	Count     int                          `json:"count"`
}

// ServiceEndpoint represents a resolved service endpoint for load balancing
type ServiceEndpoint struct {
	ClusterName string
	Service     RemoteService
	ClusterIP   netip.Addr
	TailnetIP   string
}

// LBStatus represents the status of a single service key in the load balancer
type LBStatus struct {
	ServiceKey  string            `json:"serviceKey"`
	Endpoints   []ServiceEndpoint `json:"endpoints"`
	NextIdx     uint64            `json:"nextIdx"`
	EndpointNum int               `json:"endpointNum"`
}
