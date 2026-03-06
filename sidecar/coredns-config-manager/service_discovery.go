package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// ServiceDiscovery discovers and caches services from remote clusters
type ServiceDiscovery struct {
	client       *http.Client
	peerLister   PeerLister
	serviceCache map[string]*RemoteServiceList // clusterName -> service list
	cacheMu      sync.RWMutex
	lastUpdate   map[string]time.Time
	updateMu     sync.RWMutex
}

// NewServiceDiscovery creates a new ServiceDiscovery instance
func NewServiceDiscovery(peerLister PeerLister) *ServiceDiscovery {
	return &ServiceDiscovery{
		client: &http.Client{
			Timeout: ServiceDiscoveryTimeout,
		},
		peerLister:   peerLister,
		serviceCache: make(map[string]*RemoteServiceList),
		lastUpdate:   make(map[string]time.Time),
	}
}

// DiscoverServices fetches service lists from all remote peers
func (sd *ServiceDiscovery) DiscoverServices(ctx context.Context) error {
	peers, err := sd.peerLister.GetPeers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get peers: %w", err)
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(peers))

	for _, peer := range peers {
		wg.Add(1)
		go func(p PeerInfo) {
			defer wg.Done()
			if err := sd.fetchServicesFromPeer(ctx, p); err != nil {
				klog.Errorf("Failed to fetch services from peer %s: %v", p.HostName, err)
				errChan <- err
			}
		}(peer)
	}

	wg.Wait()
	close(errChan)

	// Log errors but don't fail the entire operation
	for err := range errChan {
		klog.V(4).Infof("Service discovery error: %v", err)
	}

	klog.Infof("Service discovery completed, cached %d clusters", len(sd.serviceCache))
	return nil
}

// fetchServicesFromPeer fetches service list from a single peer
func (sd *ServiceDiscovery) fetchServicesFromPeer(ctx context.Context, peer PeerInfo) error {
	if !peer.Online {
		klog.V(4).Infof("Skipping offline peer: %s", peer.HostName)
		return nil
	}

	// Extract cluster name from hostname
	clusterName, err := extractGatewayHostName(peer.HostName)
	if err != nil {
		// If not a gateway, use hostname as-is
		clusterName = peer.HostName
	}

	// Get the first Tailscale IP
	if len(peer.TailscaleIPs) == 0 {
		return fmt.Errorf("no Tailscale IPs available")
	}

	ip := peer.TailscaleIPs[0]
	url := fmt.Sprintf("http://%s:%d%s", ip, ServiceDiscoveryPort, ServiceDiscoveryEndpoint)

	klog.V(4).Infof("Fetching services from %s (%s)", clusterName, url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := sd.client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var serviceList RemoteServiceList
	if err := json.Unmarshal(body, &serviceList); err != nil {
		return fmt.Errorf("failed to unmarshal service list: %w", err)
	}

	// Update cache
	sd.cacheMu.Lock()
	sd.serviceCache[clusterName] = &serviceList
	sd.cacheMu.Unlock()

	sd.updateMu.Lock()
	sd.lastUpdate[clusterName] = time.Now()
	sd.updateMu.Unlock()

	klog.Infof("Fetched %d services from cluster %s", serviceList.Count, clusterName)
	return nil
}

// GetServiceEndpoints returns all endpoints for a given service name and namespace
func (sd *ServiceDiscovery) GetServiceEndpoints(serviceName, namespace string) []ServiceEndpoint {
	sd.cacheMu.RLock()
	defer sd.cacheMu.RUnlock()

	var endpoints []ServiceEndpoint
	fullServiceName := fmt.Sprintf("%s.%s", serviceName, namespace)

	for clusterName, serviceList := range sd.serviceCache {
		// Look for the service in the remote cluster's service list
		// The key format in the service list is "<service-name>.<namespace>"
		if services, ok := serviceList.Services[fullServiceName]; ok {
			for _, svc := range services {
				// Parse the ClusterIP to get the IP address
				ip, err := netip.ParseAddr(svc.ClusterIP)
				if err != nil {
					klog.Warningf("Invalid ClusterIP %s for service %s in cluster %s: %v",
						svc.ClusterIP, fullServiceName, clusterName, err)
					continue
				}

				endpoints = append(endpoints, ServiceEndpoint{
					ClusterName: clusterName,
					Service:     svc,
					IP:          ip,
				})
			}
		}
	}

	klog.V(4).Infof("Found %d endpoints for service %s.%s", len(endpoints), serviceName, namespace)
	return endpoints
}

// GetCachedClusters returns list of clusters with cached service information
func (sd *ServiceDiscovery) GetCachedClusters() []string {
	sd.cacheMu.RLock()
	defer sd.cacheMu.RUnlock()

	clusters := make([]string, 0, len(sd.serviceCache))
	for clusterName := range sd.serviceCache {
		clusters = append(clusters, clusterName)
	}
	return clusters
}

// GetLastUpdateTime returns the last update time for a cluster
func (sd *ServiceDiscovery) GetLastUpdateTime(clusterName string) (time.Time, bool) {
	sd.updateMu.RLock()
	defer sd.updateMu.RUnlock()

	t, ok := sd.lastUpdate[clusterName]
	return t, ok
}

// ClearCache clears the service cache
func (sd *ServiceDiscovery) ClearCache() {
	sd.cacheMu.Lock()
	defer sd.cacheMu.Unlock()
	sd.updateMu.Lock()
	defer sd.updateMu.Unlock()

	sd.serviceCache = make(map[string]*RemoteServiceList)
	sd.lastUpdate = make(map[string]time.Time)
	klog.Info("Service discovery cache cleared")
}

// IsClustersetRemoteDomain checks if a domain matches the .clusterset.remote pattern
func IsClustersetRemoteDomain(domain string) bool {
	// Remove trailing dot if present
	domain = strings.TrimSuffix(domain, ".")

	// Check if domain ends with .clusterset.remote
	return strings.HasSuffix(domain, ".clusterset.remote")
}

// ParseClustersetDomain extracts service name and namespace from a .clusterset.remote domain
// Format: <service-name>.<namespace>.svc.clusterset.remote
// Returns: serviceName, namespace, ok
func ParseClustersetDomain(domain string) (string, string, bool) {
	// Remove trailing dot if present
	domain = strings.TrimSuffix(domain, ".")

	// Expected format: <service-name>.<namespace>.svc.clusterset.remote
	// We need at least: name.namespace.svc.clusterset.remote (5 parts)
	parts := strings.Split(domain, ".")
	if len(parts) < 5 {
		return "", "", false
	}

	// Check suffix
	if parts[len(parts)-1] != "remote" ||
		parts[len(parts)-2] != "clusterset" ||
		parts[len(parts)-3] != "svc" {
		return "", "", false
	}

	// Extract namespace and service name
	namespace := parts[len(parts)-4]
	serviceName := parts[len(parts)-5]

	// Handle cases where service name might contain dots (rejoin remaining parts)
	if len(parts) > 5 {
		serviceName = strings.Join(parts[:len(parts)-4], ".")
	}

	return serviceName, namespace, true
}
