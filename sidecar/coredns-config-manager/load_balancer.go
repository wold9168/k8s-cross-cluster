package main

import (
	"context"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/miekg/dns"
	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
	"k8s.io/klog/v2"
)

// LoadBalancer handles load balancing for .clusterset.remote domains
type LoadBalancer struct {
	serviceDiscovery *ServiceDiscovery
	dnsServer        *dnsserver.DNSServer
	// Round-robin counter per service key (serviceName.namespace)
	rrCounters map[string]*uint64
	rrMu       sync.RWMutex
}

// NewLoadBalancer creates a new LoadBalancer instance
func NewLoadBalancer(serviceDiscovery *ServiceDiscovery, dnsServer *dnsserver.DNSServer) *LoadBalancer {
	return &LoadBalancer{
		serviceDiscovery: serviceDiscovery,
		dnsServer:        dnsServer,
		rrCounters:       make(map[string]*uint64),
	}
}

// HandleQuery handles DNS queries for .clusterset.remote domains with load balancing
// Returns true if the query was handled, false otherwise
func (lb *LoadBalancer) HandleQuery(domain string, qtype uint16) ([]dns.RR, bool) {
	// Check if this is a .clusterset.remote domain
	if !IsClustersetRemoteDomain(domain) {
		return nil, false
	}

	// Parse the domain to extract service name and namespace
	serviceName, namespace, ok := ParseClustersetDomain(domain)
	if !ok {
		klog.V(4).Infof("Invalid clusterset domain format: %s", domain)
		return nil, false
	}

	klog.V(4).Infof("Load balancing query for service: %s.%s", serviceName, namespace)

	// Get all available endpoints for this service
	endpoints := lb.serviceDiscovery.GetServiceEndpoints(serviceName, namespace)
	if len(endpoints) == 0 {
		klog.V(4).Infof("No endpoints found for service: %s.%s", serviceName, namespace)
		return nil, false
	}

	// Select an endpoint using round-robin
	selectedEndpoint := lb.selectEndpoint(serviceName, namespace, endpoints)
	if selectedEndpoint == nil {
		return nil, false
	}

	// Build DNS response based on query type
	answers := lb.buildAnswer(domain, qtype, *selectedEndpoint)
	if len(answers) == 0 {
		return nil, false
	}

	klog.Infof("Load balanced query %s -> %s (cluster: %s)",
		domain, selectedEndpoint.IP, selectedEndpoint.ClusterName)

	return answers, true
}

// selectEndpoint selects an endpoint using round-robin load balancing
func (lb *LoadBalancer) selectEndpoint(serviceName, namespace string, endpoints []ServiceEndpoint) *ServiceEndpoint {
	if len(endpoints) == 0 {
		return nil
	}

	if len(endpoints) == 1 {
		return &endpoints[0]
	}

	// Get or create counter for this service
	serviceKey := serviceName + "." + namespace
	counter := lb.getCounter(serviceKey)

	// Increment counter and select endpoint
	idx := atomic.AddUint64(counter, 1) % uint64(len(endpoints))
	return &endpoints[idx]
}

// getCounter gets or creates a round-robin counter for a service
func (lb *LoadBalancer) getCounter(serviceKey string) *uint64 {
	lb.rrMu.RLock()
	counter, ok := lb.rrCounters[serviceKey]
	lb.rrMu.RUnlock()

	if ok {
		return counter
	}

	// Create new counter
	lb.rrMu.Lock()
	defer lb.rrMu.Unlock()

	// Double-check after acquiring write lock
	if counter, ok = lb.rrCounters[serviceKey]; ok {
		return counter
	}

	var newCounter uint64
	lb.rrCounters[serviceKey] = &newCounter
	return &newCounter
}

// buildAnswer builds DNS response records for the selected endpoint
func (lb *LoadBalancer) buildAnswer(domain string, qtype uint16, endpoint ServiceEndpoint) []dns.RR {
	var answers []dns.RR

	switch qtype {
	case dns.TypeA:
		if endpoint.IP.Is4() {
			rr, err := dns.NewRR(domain + " 60 IN A " + endpoint.IP.String())
			if err != nil {
				klog.Errorf("Failed to create A record: %v", err)
				return nil
			}
			answers = append(answers, rr)
		}
	case dns.TypeAAAA:
		if endpoint.IP.Is6() {
			rr, err := dns.NewRR(domain + " 60 IN AAAA " + endpoint.IP.String())
			if err != nil {
				klog.Errorf("Failed to create AAAA record: %v", err)
				return nil
			}
			answers = append(answers, rr)
		}
	case dns.TypeANY:
		// For ANY queries, return both A and AAAA if available
		if endpoint.IP.Is4() {
			rr, err := dns.NewRR(domain + " 60 IN A " + endpoint.IP.String())
			if err != nil {
				klog.Errorf("Failed to create A record: %v", err)
			} else {
				answers = append(answers, rr)
			}
		} else if endpoint.IP.Is6() {
			rr, err := dns.NewRR(domain + " 60 IN AAAA " + endpoint.IP.String())
			if err != nil {
				klog.Errorf("Failed to create AAAA record: %v", err)
			} else {
				answers = append(answers, rr)
			}
		}
	}

	return answers
}

// RefreshServices triggers service discovery to refresh the cache
func (lb *LoadBalancer) RefreshServices(ctx context.Context) error {
	return lb.serviceDiscovery.DiscoverServices(ctx)
}

// GetServiceCount returns the number of unique services discovered across all clusters
func (lb *LoadBalancer) GetServiceCount() int {
	lb.serviceDiscovery.cacheMu.RLock()
	defer lb.serviceDiscovery.cacheMu.RUnlock()

	count := 0
	for _, serviceList := range lb.serviceDiscovery.serviceCache {
		count += serviceList.Count
	}
	return count
}

// GetClusterCount returns the number of clusters with cached service information
func (lb *LoadBalancer) GetClusterCount() int {
	return len(lb.serviceDiscovery.GetCachedClusters())
}

// RegisterDNSServer registers a DNS server handler for load balanced queries
// This integrates the load balancer with the embedded DNS server
func (lb *LoadBalancer) RegisterDNSServer() {
	// The load balancer is integrated via the DNSConfigManager
	// which calls HandleQuery for each DNS request
	klog.Info("Load balancer registered with DNS server")
}

// IsServiceAvailable checks if a service is available in any remote cluster
func (lb *LoadBalancer) IsServiceAvailable(serviceName, namespace string) bool {
	endpoints := lb.serviceDiscovery.GetServiceEndpoints(serviceName, namespace)
	return len(endpoints) > 0
}

// GetAvailableServices returns a list of all available services across all clusters
func (lb *LoadBalancer) GetAvailableServices() []string {
	lb.serviceDiscovery.cacheMu.RLock()
	defer lb.serviceDiscovery.cacheMu.RUnlock()

	serviceSet := make(map[string]bool)
	for _, serviceList := range lb.serviceDiscovery.serviceCache {
		for serviceName := range serviceList.Services {
			serviceSet[serviceName] = true
		}
	}

	services := make([]string, 0, len(serviceSet))
	for svc := range serviceSet {
		services = append(services, svc)
	}
	return services
}

// GetEndpointCountForService returns the number of endpoints for a specific service
func (lb *LoadBalancer) GetEndpointCountForService(serviceName, namespace string) int {
	endpoints := lb.serviceDiscovery.GetServiceEndpoints(serviceName, namespace)
	return len(endpoints)
}

// ResolveService resolves a service name to an IP address using load balancing
// This is a helper method that can be used by other components
func (lb *LoadBalancer) ResolveService(serviceName, namespace string, qtype uint16) (netip.Addr, string, bool) {
	endpoints := lb.serviceDiscovery.GetServiceEndpoints(serviceName, namespace)
	if len(endpoints) == 0 {
		return netip.Addr{}, "", false
	}

	endpoint := lb.selectEndpoint(serviceName, namespace, endpoints)
	if endpoint == nil {
		return netip.Addr{}, "", false
	}

	// Filter by query type
	switch qtype {
	case dns.TypeA:
		if !endpoint.IP.Is4() {
			return netip.Addr{}, "", false
		}
	case dns.TypeAAAA:
		if !endpoint.IP.Is6() {
			return netip.Addr{}, "", false
		}
	}

	return endpoint.IP, endpoint.ClusterName, true
}

// FormatClustersetDomain creates a .clusterset.remote domain from service name and namespace
func FormatClustersetDomain(serviceName, namespace string) string {
	// Normalize service name by replacing dots with hyphens for DNS compatibility
	serviceName = strings.ReplaceAll(serviceName, ".", "-")
	return serviceName + "." + namespace + ".svc.clusterset.remote"
}
