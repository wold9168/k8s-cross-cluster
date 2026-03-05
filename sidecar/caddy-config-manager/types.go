package main

// DomainMapping represents the mapping between remote and local domains
type DomainMapping struct {
	RemoteDomain string
	LocalDomain  string
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
