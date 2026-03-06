package main

import (
	"context"
	"net/netip"
	"sync"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"

	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
)

func TestLoadBalancer_New(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")

	lb := NewLoadBalancer(sd, dnsSrv)

	assert.NotNil(t, lb)
	assert.Equal(t, sd, lb.serviceDiscovery)
	assert.Equal(t, dnsSrv, lb.dnsServer)
	assert.NotNil(t, lb.rrCounters)
	assert.Len(t, lb.rrCounters, 0)
}

func TestLoadBalancer_HandleQuery_NotClustersetDomain(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	answers, handled := lb.HandleQuery("myapp.default.svc.cluster.local", dns.TypeA)
	assert.False(t, handled)
	assert.Nil(t, answers)
}

func TestLoadBalancer_HandleQuery_NoEndpoints(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	answers, handled := lb.HandleQuery("myapp.default.svc.clusterset.remote", dns.TypeA)
	assert.False(t, handled)
	assert.Nil(t, answers)
}

func TestLoadBalancer_HandleQuery_SingleEndpoint(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	// Populate service discovery cache
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
			Count: 1,
		},
	}

	answers, handled := lb.HandleQuery("myapp.default.svc.clusterset.remote", dns.TypeA)
	assert.True(t, handled)
	assert.Len(t, answers, 1)
	assert.Contains(t, answers[0].String(), "10.96.1.10")
}

func TestLoadBalancer_HandleQuery_MultipleEndpoints_RoundRobin(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	// Populate service discovery cache with multiple endpoints
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
			Count: 1,
		},
		"cluster2": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.2.10"},
				},
			},
			Count: 1,
		},
		"cluster3": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.3.10"},
				},
			},
			Count: 1,
		},
	}

	// Make multiple queries to test round-robin
	ips := make(map[string]bool)
	for i := 0; i < 6; i++ {
		answers, handled := lb.HandleQuery("myapp.default.svc.clusterset.remote", dns.TypeA)
		assert.True(t, handled)
		assert.Len(t, answers, 1)
		
		// Extract IP from answer
		ip := answers[0].(*dns.A).A.String()
		ips[ip] = true
	}

	// Should have seen all 3 IPs
	assert.Len(t, ips, 3)
	assert.True(t, ips["10.96.1.10"])
	assert.True(t, ips["10.96.2.10"])
	assert.True(t, ips["10.96.3.10"])
}

func TestLoadBalancer_HandleQuery_AAAA(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	// Populate with IPv6 address
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "fd7a:115c:a1e0::1"},
				},
			},
			Count: 1,
		},
	}

	answers, handled := lb.HandleQuery("myapp.default.svc.clusterset.remote", dns.TypeAAAA)
	assert.True(t, handled)
	assert.Len(t, answers, 1)
	assert.Contains(t, answers[0].String(), "fd7a:115c:a1e0::1")
}

func TestLoadBalancer_HandleQuery_TypeMismatch(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	// Populate with IPv4 address
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
			Count: 1,
		},
	}

	// Query for AAAA but service only has IPv4
	answers, handled := lb.HandleQuery("myapp.default.svc.clusterset.remote", dns.TypeAAAA)
	assert.False(t, handled)
	assert.Nil(t, answers)
}

func TestLoadBalancer_HandleQuery_ANY(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	// Populate with IPv4 address
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
			Count: 1,
		},
	}

	answers, handled := lb.HandleQuery("myapp.default.svc.clusterset.remote", dns.TypeANY)
	assert.True(t, handled)
	assert.Len(t, answers, 1)
	assert.Contains(t, answers[0].String(), "10.96.1.10")
}

func TestLoadBalancer_HandleQuery_InvalidDomain(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	// Populate cache
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
			Count: 1,
		},
	}

	// Invalid domain format
	answers, handled := lb.HandleQuery("invalid.clusterset.remote", dns.TypeA)
	assert.False(t, handled)
	assert.Nil(t, answers)
}

func TestLoadBalancer_selectEndpoint_Single(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	endpoints := []ServiceEndpoint{
		{ClusterName: "cluster1", IP: netip.MustParseAddr("10.96.1.10")},
	}

	result := lb.selectEndpoint("myapp", "default", endpoints)
	assert.NotNil(t, result)
	assert.Equal(t, "cluster1", result.ClusterName)
}

func TestLoadBalancer_selectEndpoint_Multiple(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	endpoints := []ServiceEndpoint{
		{ClusterName: "cluster1", IP: netip.MustParseAddr("10.96.1.10")},
		{ClusterName: "cluster2", IP: netip.MustParseAddr("10.96.2.10")},
		{ClusterName: "cluster3", IP: netip.MustParseAddr("10.96.3.10")},
	}

	// Test round-robin behavior
	selected := make(map[string]bool)
	for i := 0; i < 6; i++ {
		result := lb.selectEndpoint("myapp", "default", endpoints)
		assert.NotNil(t, result)
		selected[result.ClusterName] = true
	}

	// Should have selected all endpoints
	assert.Len(t, selected, 3)
}

func TestLoadBalancer_selectEndpoint_Empty(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	endpoints := []ServiceEndpoint{}
	result := lb.selectEndpoint("myapp", "default", endpoints)
	assert.Nil(t, result)
}

func TestLoadBalancer_getCounter_New(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	counter := lb.getCounter("myapp.default")
	assert.NotNil(t, counter)
	assert.Equal(t, uint64(0), *counter)
}

func TestLoadBalancer_getCounter_Existing(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	// Get counter first time
	counter1 := lb.getCounter("myapp.default")
	*counter1 = 42

	// Get counter second time
	counter2 := lb.getCounter("myapp.default")
	assert.Equal(t, counter1, counter2)
	assert.Equal(t, uint64(42), *counter2)
}

func TestLoadBalancer_getCounter_Concurrent(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter := lb.getCounter("myapp.default")
			assert.NotNil(t, counter)
		}()
	}
	wg.Wait()
}

func TestLoadBalancer_buildAnswer_A(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	endpoint := ServiceEndpoint{
		ClusterName: "cluster1",
		IP:          netip.MustParseAddr("10.96.1.10"),
	}

	answers := lb.buildAnswer("myapp.default.svc.clusterset.remote", dns.TypeA, endpoint)
	assert.Len(t, answers, 1)
	assert.Contains(t, answers[0].String(), "10.96.1.10")
}

func TestLoadBalancer_buildAnswer_AAAA(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	endpoint := ServiceEndpoint{
		ClusterName: "cluster1",
		IP:          netip.MustParseAddr("fd7a:115c:a1e0::1"),
	}

	answers := lb.buildAnswer("myapp.default.svc.clusterset.remote", dns.TypeAAAA, endpoint)
	assert.Len(t, answers, 1)
	assert.Contains(t, answers[0].String(), "fd7a:115c:a1e0::1")
}

func TestLoadBalancer_buildAnswer_TypeMismatch(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	// IPv4 endpoint
	endpoint := ServiceEndpoint{
		ClusterName: "cluster1",
		IP:          netip.MustParseAddr("10.96.1.10"),
	}

	// Query for AAAA
	answers := lb.buildAnswer("myapp.default.svc.clusterset.remote", dns.TypeAAAA, endpoint)
	assert.Len(t, answers, 0)
}

func TestLoadBalancer_RefreshServices(t *testing.T) {
	mockPeerLister := &MockPeerLister{
		peers: []PeerInfo{},
	}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	err := lb.RefreshServices(context.Background())
	assert.NoError(t, err)
}

func TestLoadBalancer_GetServiceCount(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {Count: 5},
		"cluster2": {Count: 3},
	}

	count := lb.GetServiceCount()
	assert.Equal(t, 8, count)
}

func TestLoadBalancer_GetClusterCount(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {Count: 1},
		"cluster2": {Count: 2},
		"cluster3": {Count: 3},
	}

	count := lb.GetClusterCount()
	assert.Equal(t, 3, count)
}

func TestLoadBalancer_IsServiceAvailable(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	// Service exists
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
		},
	}

	assert.True(t, lb.IsServiceAvailable("myapp", "default"))
	assert.False(t, lb.IsServiceAvailable("other", "default"))
}

func TestLoadBalancer_GetAvailableServices(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {},
				"api.default":   {},
			},
		},
		"cluster2": {
			Services: map[string][]RemoteService{
				"myapp.default": {},
				"db.production": {},
			},
		},
	}

	services := lb.GetAvailableServices()
	assert.Len(t, services, 3)
	assert.Contains(t, services, "myapp.default")
	assert.Contains(t, services, "api.default")
	assert.Contains(t, services, "db.production")
}

func TestLoadBalancer_GetEndpointCountForService(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.11"},
				},
			},
		},
		"cluster2": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.2.10"},
				},
			},
		},
	}

	count := lb.GetEndpointCountForService("myapp", "default")
	assert.Equal(t, 3, count)
}

func TestLoadBalancer_ResolveService(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
		},
	}

	ip, cluster, ok := lb.ResolveService("myapp", "default", dns.TypeA)
	assert.True(t, ok)
	assert.Equal(t, "10.96.1.10", ip.String())
	assert.Equal(t, "cluster1", cluster)
}

func TestLoadBalancer_ResolveService_NotFound(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	lb := NewLoadBalancer(sd, dnsSrv)

	ip, cluster, ok := lb.ResolveService("myapp", "default", dns.TypeA)
	assert.False(t, ok)
	assert.Equal(t, netip.Addr{}, ip)
	assert.Empty(t, cluster)
}

func TestFormatClustersetDomain(t *testing.T) {
	tests := []struct {
		name      string
		service   string
		namespace string
		want      string
	}{
		{"simple", "myapp", "default", "myapp.default.svc.clusterset.remote"},
		{"with dots", "my.app", "default", "my-app.default.svc.clusterset.remote"},
		{"production", "api", "production", "api.production.svc.clusterset.remote"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatClustersetDomain(tt.service, tt.namespace)
			assert.Equal(t, tt.want, got)
		})
	}
}
