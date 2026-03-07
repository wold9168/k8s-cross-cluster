package main

// DomainMapping represents the mapping between remote and local domains
type DomainMapping struct {
	RemoteDomain string // remote domain: <service>.<namespace>.svc.clusterset.remote or <service>.<namespace>.svc.<cluster>.remote
	LocalDomain  string // local domain: <service>.<namespace>.svc.cluster.local
}

// DomainMappingResult holds the result of domain generation
type DomainMappingResult struct {
	RemoteDomains []string
	DomainMapping map[string]string
}

// CaddyConfig represents a generated Caddy configuration block
type CaddyConfig struct {
	RemoteDomain string
	LocalDomain  string
	ConfigBlock  string
}
