package main

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
