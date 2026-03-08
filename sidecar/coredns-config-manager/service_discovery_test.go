package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockPeerLister implements PeerLister for testing
type MockPeerLister struct {
	peers []PeerInfo
	self  PeerInfo
	err   error
}

func (m *MockPeerLister) GetPeers(ctx context.Context) ([]PeerInfo, error) {
	return m.peers, m.err
}

func (m *MockPeerLister) GetSelf(ctx context.Context) (PeerInfo, error) {
	return m.self, m.err
}

func TestServiceDiscovery_New(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)

	assert.NotNil(t, sd)
	assert.Equal(t, mockPeerLister, sd.peerLister)
	assert.NotNil(t, sd.client)
	assert.Equal(t, ServiceDiscoveryTimeout, sd.client.Timeout)
	assert.Len(t, sd.serviceCache, 0)
}

func TestServiceDiscovery_DiscoverServices(t *testing.T) {
	// This test verifies the concurrent fetching logic
	// Actual HTTP integration is tested in fetchServicesFromPeer tests
	
	mockPeerLister := &MockPeerLister{
		peers: []PeerInfo{
			{
				HostName:     "cluster1-tsgateway",
				TailscaleIPs: []string{"100.64.0.1"},
				Online:       true,
			},
			{
				HostName:     "cluster2-tsgateway",
				TailscaleIPs: []string{"100.64.0.2"},
				Online:       true,
			},
		},
	}

	sd := NewServiceDiscovery(mockPeerLister)
	// This will fail to connect (no actual server), but tests the concurrent logic
	err := sd.DiscoverServices(context.Background())

	// Error is expected since there's no actual server, but the function should complete
	// and log errors without failing
	assert.NoError(t, err)
}

func TestServiceDiscovery_DiscoverServices_OfflinePeer(t *testing.T) {
	mockPeerLister := &MockPeerLister{
		peers: []PeerInfo{
			{
				HostName:     "cluster1-tsgateway",
				TailscaleIPs: []string{"100.64.0.1"},
				Online:       false, // Offline peer
			},
		},
	}

	sd := NewServiceDiscovery(mockPeerLister)
	err := sd.DiscoverServices(context.Background())

	assert.NoError(t, err)
	// Should skip offline peers, cache should be empty
	assert.Len(t, sd.serviceCache, 0)
}

func TestServiceDiscovery_DiscoverServices_NoIPs(t *testing.T) {
	mockPeerLister := &MockPeerLister{
		peers: []PeerInfo{
			{
				HostName:     "cluster1-tsgateway",
				TailscaleIPs: []string{}, // No IPs
				Online:       true,
			},
		},
	}

	sd := NewServiceDiscovery(mockPeerLister)
	err := sd.DiscoverServices(context.Background())

	// Should not fail entirely, but should log error
	assert.NoError(t, err)
}

func TestServiceDiscovery_DiscoverServices_Error(t *testing.T) {
	mockPeerLister := &MockPeerLister{
		err: assert.AnError,
	}

	sd := NewServiceDiscovery(mockPeerLister)
	err := sd.DiscoverServices(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get peers")
}

func TestServiceDiscovery_GetServiceEndpoints(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)

	// Populate cache with test data
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
	}

	endpoints := sd.GetServiceEndpoints("myapp", "default")

	assert.Len(t, endpoints, 2)
	
	// Check that both clusters are represented
	clusterNames := make(map[string]bool)
	for _, ep := range endpoints {
		clusterNames[ep.ClusterName] = true
		assert.NotNil(t, ep.ClusterIP)
	}
	assert.True(t, clusterNames["cluster1"])
	assert.True(t, clusterNames["cluster2"])
}

func TestServiceDiscovery_GetServiceEndpoints_NotFound(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)

	// Populate cache with different service
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"other.default": {
					{Name: "other", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
			Count: 1,
		},
	}

	endpoints := sd.GetServiceEndpoints("myapp", "default")
	assert.Len(t, endpoints, 0)
}

func TestServiceDiscovery_GetServiceEndpoints_InvalidIP(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)

	// Populate cache with invalid IP
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "invalid-ip"},
				},
			},
			Count: 1,
		},
	}

	endpoints := sd.GetServiceEndpoints("myapp", "default")
	// Should skip invalid IP
	assert.Len(t, endpoints, 0)
}

func TestServiceDiscovery_GetCachedClusters(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)

	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {Count: 1},
		"cluster2": {Count: 2},
		"cluster3": {Count: 3},
	}

	clusters := sd.GetCachedClusters()
	assert.Len(t, clusters, 3)
	assert.Contains(t, clusters, "cluster1")
	assert.Contains(t, clusters, "cluster2")
	assert.Contains(t, clusters, "cluster3")
}

func TestServiceDiscovery_GetLastUpdateTime(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)

	before := time.Now()
	sd.lastUpdate["cluster1"] = before
	after := time.Now()

	tm, ok := sd.GetLastUpdateTime("cluster1")
	assert.True(t, ok)
	assert.WithinDuration(t, before, tm, after.Sub(before))

	_, ok = sd.GetLastUpdateTime("nonexistent")
	assert.False(t, ok)
}

func TestServiceDiscovery_ClearCache(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)

	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {Count: 1},
	}
	sd.lastUpdate = map[string]time.Time{
		"cluster1": time.Now(),
	}

	sd.ClearCache()

	assert.Len(t, sd.serviceCache, 0)
	assert.Len(t, sd.lastUpdate, 0)
}

func TestIsClustersetRemoteDomain(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		want   bool
	}{
		{"valid", "myapp.default.svc.clusterset.remote", true},
		{"valid with trailing dot", "myapp.default.svc.clusterset.remote.", true},
		{"invalid no clusterset", "myapp.default.svc.remote", false},
		{"invalid no remote", "myapp.default.svc.clusterset", false},
		{"invalid different suffix", "myapp.default.svc.cluster.local", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsClustersetRemoteDomain(tt.domain)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseClustersetDomain(t *testing.T) {
	tests := []struct {
		name          string
		domain        string
		wantService   string
		wantNamespace string
		wantOk        bool
	}{
		{
			name:          "valid simple",
			domain:        "myapp.default.svc.clusterset.remote",
			wantService:   "myapp",
			wantNamespace: "default",
			wantOk:        true,
		},
		{
			name:          "valid with trailing dot",
			domain:        "myapp.default.svc.clusterset.remote.",
			wantService:   "myapp",
			wantNamespace: "default",
			wantOk:        true,
		},
		{
			name:          "valid different namespace",
			domain:        "api.production.svc.clusterset.remote",
			wantService:   "api",
			wantNamespace: "production",
			wantOk:        true,
		},
		{
			name:          "invalid no svc",
			domain:        "myapp.default.clusterset.remote",
			wantService:   "",
			wantNamespace: "",
			wantOk:        false,
		},
		{
			name:          "invalid too short",
			domain:        "myapp.default.remote",
			wantService:   "",
			wantNamespace: "",
			wantOk:        false,
		},
		{
			name:          "invalid wrong suffix",
			domain:        "myapp.default.svc.cluster.local",
			wantService:   "",
			wantNamespace: "",
			wantOk:        false,
		},
		{
			name:          "empty",
			domain:        "",
			wantService:   "",
			wantNamespace: "",
			wantOk:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotService, gotNamespace, gotOk := ParseClustersetDomain(tt.domain)
			assert.Equal(t, tt.wantOk, gotOk)
			if gotOk {
				assert.Equal(t, tt.wantService, gotService)
				assert.Equal(t, tt.wantNamespace, gotNamespace)
			}
		})
	}
}

func TestParseClustersetDomain_WithDots(t *testing.T) {
	// Test service names with dots (should be rejoined)
	service, namespace, ok := ParseClustersetDomain("my.app.default.svc.clusterset.remote")
	assert.True(t, ok)
	assert.Equal(t, "my.app", service)
	assert.Equal(t, "default", namespace)
}

func TestServiceDiscovery_fetchServicesFromPeer_HTTPError(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)

	// Create peer with unreachable IP
	peer := PeerInfo{
		HostName:     "cluster1-tsgateway",
		TailscaleIPs: []string{"192.0.2.1"}, // TEST-NET-1, should be unreachable
		Online:       true,
	}

	err := sd.fetchServicesFromPeer(context.Background(), peer)
	assert.Error(t, err)
}

func TestServiceDiscovery_fetchServicesFromPeer_Offline(t *testing.T) {
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)

	// Offline peer should be skipped
	peer := PeerInfo{
		HostName:     "cluster1-tsgateway",
		TailscaleIPs: []string{"100.64.0.1"},
		Online:       false,
	}

	err := sd.fetchServicesFromPeer(context.Background(), peer)
	assert.NoError(t, err)
}
