package main

import (
	"context"
	"fmt"
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
	peerLister       PeerLister
	peerCache        map[string]PeerInfo // clusterName -> PeerInfo
	// Round-robin counter per service key (serviceName.namespace)
	rrCounters map[string]*uint64
	rrMu       sync.RWMutex
}

// NewLoadBalancer creates a new LoadBalancer instance
func NewLoadBalancer(serviceDiscovery *ServiceDiscovery, dnsServer *dnsserver.DNSServer, peerLister PeerLister) *LoadBalancer {
	return &LoadBalancer{
		serviceDiscovery: serviceDiscovery,
		dnsServer:        dnsServer,
		peerLister:       peerLister,
		peerCache:        make(map[string]PeerInfo),
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

	// Get Tailscale IP for logging
	tailnetIP := lb.GetPeerTailscaleIP(selectedEndpoint.ClusterName)
	if tailnetIP == "" {
		tailnetIP = selectedEndpoint.ClusterIP.String()
	}
	klog.Infof("Load balanced query %s -> %s (cluster: %s)",
		domain, tailnetIP, selectedEndpoint.ClusterName)

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

	// Create new counter using new() to allocate on heap
	// Need to acquire write lock to prevent race condition
	lb.rrMu.Lock()
	defer lb.rrMu.Unlock()

	// Double-check after acquiring write lock
	if counter, ok = lb.rrCounters[serviceKey]; ok {
		return counter
	}

	counter = new(uint64)
	lb.rrCounters[serviceKey] = counter
	return counter
}

// buildAnswer builds DNS response records for the selected endpoint
// Uses the Tailscale IP of the remote cluster instead of the endpoint's direct IP
func (lb *LoadBalancer) buildAnswer(domain string, qtype uint16, endpoint ServiceEndpoint) []dns.RR {
	var answers []dns.RR

	// Get Tailscale IP for the cluster where this endpoint resides
	tailscaleIP := lb.GetPeerTailscaleIP(endpoint.ClusterName)
	if tailscaleIP == "" {
		klog.Warningf("No Tailscale IP found for cluster %s, falling back to endpoint IP", endpoint.ClusterName)
		tailscaleIP = endpoint.ClusterIP.String()
	}

	switch qtype {
	case dns.TypeA:
		if endpoint.ClusterIP.Is4() || tailscaleIP != "" {
			rr, err := dns.NewRR(domain + " 60 IN A " + tailscaleIP)
			if err != nil {
				klog.Errorf("Failed to create A record: %v", err)
				return nil
			}
			answers = append(answers, rr)
		}
	case dns.TypeAAAA:
		if endpoint.ClusterIP.Is6() || tailscaleIP != "" {
			rr, err := dns.NewRR(domain + " 60 IN AAAA " + tailscaleIP)
			if err != nil {
				klog.Errorf("Failed to create AAAA record: %v", err)
				return nil
			}
			answers = append(answers, rr)
		}
	case dns.TypeANY:
		// For ANY queries, return both A and AAAA if available
		if tailscaleIP != "" {
			rr, err := dns.NewRR(domain + " 60 IN A " + tailscaleIP)
			if err != nil {
				klog.Errorf("Failed to create A record: %v", err)
			} else {
				answers = append(answers, rr)
			}
		}
	}

	return answers
}

// RefreshServices triggers service discovery to refresh the cache
func (lb *LoadBalancer) RefreshServices(ctx context.Context) error {
	// Refresh peer cache first
	if err := lb.refreshPeerCache(ctx); err != nil {
		klog.Warningf("Failed to refresh peer cache: %v", err)
	}

	return lb.serviceDiscovery.DiscoverServices(ctx)
}

// refreshPeerCache updates the peer cache with latest Tailscale peer information
func (lb *LoadBalancer) refreshPeerCache(ctx context.Context) error {
	if lb.peerLister == nil {
		return nil
	}

	peers, err := lb.peerLister.GetPeers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get peers: %w", err)
	}

	// Also get self peer
	self, err := lb.peerLister.GetSelf(ctx)
	if err != nil {
		klog.V(4).Infof("Failed to get self peer: %v", err)
	} else {
		lb.updatePeerCache(self)
	}

	// Update cache with all peers
	for _, peer := range peers {
		lb.updatePeerCache(peer)
	}

	klog.Infof("Peer cache refreshed: %d peers", len(lb.peerCache))
	return nil
}

// updatePeerCache updates the peer cache with a single peer's information
func (lb *LoadBalancer) updatePeerCache(peer PeerInfo) {
	clusterName, err := extractGatewayHostName(peer.HostName)
	if err != nil {
		// If not a gateway, use hostname as-is
		clusterName = peer.HostName
	}

	lb.peerCache[clusterName] = peer
	klog.V(4).Infof("Updated peer cache: cluster %s -> %v", clusterName, peer.TailscaleIPs)
}

// GetPeerTailscaleIP returns a Tailscale IP for the given cluster name
// Returns the first available IP (preferring IPv4)
func (lb *LoadBalancer) GetPeerTailscaleIP(clusterName string) string {
	peer, ok := lb.peerCache[clusterName]
	if !ok {
		return ""
	}

	// Prefer IPv4 over IPv6
	for _, ip := range peer.TailscaleIPs {
		if strings.Contains(ip, ".") {
			return ip
		}
	}

	// Return first available IP if no IPv4 found
	if len(peer.TailscaleIPs) > 0 {
		return peer.TailscaleIPs[0]
	}

	return ""
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
		if !endpoint.ClusterIP.Is4() {
			return netip.Addr{}, "", false
		}
	case dns.TypeAAAA:
		if !endpoint.ClusterIP.Is6() {
			return netip.Addr{}, "", false
		}
	}

	return endpoint.ClusterIP, endpoint.ClusterName, true
}

// FormatClustersetDomain creates a .clusterset.remote domain from service name and namespace
func FormatClustersetDomain(serviceName, namespace string) string {
	// Normalize service name by replacing dots with hyphens for DNS compatibility
	serviceName = strings.ReplaceAll(serviceName, ".", "-")
	return serviceName + "." + namespace + ".svc.clusterset.remote"
}

// GetLoadBalancerStatus returns the current status of all discovered services
// This includes all services found from remote clusters, with their endpoints and round-robin info
func (lb *LoadBalancer) GetLoadBalancerStatus() []LBStatus {
	// Get all discovered service keys from service discovery
	allServiceKeys := lb.serviceDiscovery.GetAllServiceKeys()

	status := make([]LBStatus, 0, len(allServiceKeys))

	lb.rrMu.RLock()
	defer lb.rrMu.RUnlock()

	for serviceKey := range allServiceKeys {
		// Parse service key to get service name and namespace
		parts := strings.SplitN(serviceKey, ".", 2)
		if len(parts) != 2 {
			continue
		}
		serviceName := parts[0]
		namespace := parts[1]

		// Get endpoints for this service
		endpoints := lb.serviceDiscovery.GetServiceEndpoints(serviceName, namespace)
		if len(endpoints) == 0 {
			continue
		}

		// Populate TailnetIP for each endpoint from peerCache
		for i := range endpoints {
			endpoints[i].TailnetIP = lb.GetPeerTailscaleIP(endpoints[i].ClusterName)
		}

		// Get counter if exists, otherwise use 0
		var nextIdx uint64
		counter, ok := lb.rrCounters[serviceKey]
		if ok && len(endpoints) > 1 {
			// Calculate next idx: (current counter + 1) % len(endpoints)
			currentCounter := atomic.LoadUint64(counter)
			nextIdx = (currentCounter + 1) % uint64(len(endpoints))
		}

		status = append(status, LBStatus{
			ServiceKey:  serviceKey,
			Endpoints:   endpoints,
			NextIdx:     nextIdx,
			EndpointNum: len(endpoints),
		})
	}

	return status
}

// RotateAllServices triggers round-robin rotation for all discovered services
// This forces the creation of counters for all services, even if they have only one endpoint
// Returns a map of serviceKey -> nextIdx after rotation
func (lb *LoadBalancer) RotateAllServices() map[string]uint64 {
	results := make(map[string]uint64)

	// Get all discovered service keys from service discovery
	allServiceKeys := lb.serviceDiscovery.GetAllServiceKeys()

	lb.rrMu.Lock()
	defer lb.rrMu.Unlock()

	for serviceKey := range allServiceKeys {
		// Parse service key to get service name and namespace
		parts := strings.SplitN(serviceKey, ".", 2)
		if len(parts) != 2 {
			continue
		}
		serviceName := parts[0]
		namespace := parts[1]

		// Get endpoints for this service
		endpoints := lb.serviceDiscovery.GetServiceEndpoints(serviceName, namespace)
		if len(endpoints) == 0 {
			continue
		}

		// Populate TailnetIP for each endpoint from peerCache
		for i := range endpoints {
			endpoints[i].TailnetIP = lb.GetPeerTailscaleIP(endpoints[i].ClusterName)
		}

		// Get or create counter for this service
		counter, ok := lb.rrCounters[serviceKey]
		if !ok {
			counter = new(uint64)
			lb.rrCounters[serviceKey] = counter
		}

		// Increment counter and get next idx
		if len(endpoints) > 1 {
			nextIdx := atomic.AddUint64(counter, 1) % uint64(len(endpoints))
			results[serviceKey] = nextIdx
			klog.Infof("Rotated service %s to index %d (total endpoints: %d)", serviceKey, nextIdx, len(endpoints))
		} else {
			// Single endpoint, still record it but with idx 0
			results[serviceKey] = 0
			klog.Infof("Service %s.%s has only 1 endpoint, recorded with idx 0", serviceName, namespace)
		}
	}

	klog.Infof("Rotation completed for %d services", len(results))
	return results
}
