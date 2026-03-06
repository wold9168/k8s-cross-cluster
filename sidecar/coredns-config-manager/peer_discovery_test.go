package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPeerDiscovery_New(t *testing.T) {
	pd := NewPeerDiscovery()

	assert.NotNil(t, pd)
	assert.NotNil(t, pd.client)
}

func TestExtractGatewayHostName(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     string
		wantErr  bool
	}{
		{"valid gateway", "cluster1-tsgateway", "cluster1", false},
		{"valid gateway long", "my-cluster-name-tsgateway", "my-cluster-name", false},
		{"invalid no suffix", "cluster1", "", true},
		{"invalid wrong suffix", "cluster1-gateway", "", true},
		{"invalid partial suffix", "cluster1-tsgateway-extra", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractGatewayHostName(tt.hostname)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestIsGatewayHostName(t *testing.T) {
	assert.True(t, isGatewayHostName("cluster1-tsgateway"))
	assert.True(t, isGatewayHostName("my-cluster-tsgateway"))
	assert.False(t, isGatewayHostName("cluster1"))
	assert.False(t, isGatewayHostName("cluster1-gateway"))
	assert.False(t, isGatewayHostName(""))
}

func TestPeerDiscovery_ExtractGatewayNodeName(t *testing.T) {
	pd := NewPeerDiscovery()

	peer := PeerInfo{
		HostName: "test-cluster-tsgateway",
	}

	nodeName, err := pd.ExtractGatewayNodeName(peer)

	assert.NoError(t, err)
	assert.Equal(t, "test-cluster", nodeName)
}

func TestPeerDiscovery_ExtractGatewayNodeName_Invalid(t *testing.T) {
	pd := NewPeerDiscovery()

	peer := PeerInfo{
		HostName: "invalid-hostname",
	}

	nodeName, err := pd.ExtractGatewayNodeName(peer)

	assert.Error(t, err)
	assert.Empty(t, nodeName)
}

func TestPeerDiscovery_GetPeers_Empty(t *testing.T) {
	pd := NewPeerDiscovery()
	ctx := context.Background()

	// This will fail in test environment without tailscaled
	// but tests the error handling
	peers, err := pd.GetPeers(ctx)

	// May fail due to missing tailscaled socket
	// Test just ensures the function handles errors gracefully
	if err != nil {
		assert.Empty(t, peers)
	}
}

func TestPeerDiscovery_GetSelf_Empty(t *testing.T) {
	pd := NewPeerDiscovery()
	ctx := context.Background()

	// This will fail in test environment without tailscaled
	self, err := pd.GetSelf(ctx)

	// May fail due to missing tailscaled socket
	if err != nil {
		assert.Empty(t, self.ID)
	}
}

func TestPeerDiscovery_GetGatewayPeers(t *testing.T) {
	pd := NewPeerDiscovery()
	ctx := context.Background()

	// This will fail in test environment without tailscaled
	// but tests the function exists and handles errors
	peers, err := pd.GetGatewayPeers(ctx)

	// May fail due to missing tailscaled socket
	if err != nil {
		assert.Empty(t, peers)
	}
}

func TestExtractGatewayHostName_EdgeCases(t *testing.T) {
	// Exact length of suffix
	hostname := "-tsgateway"
	result, err := extractGatewayHostName(hostname)
	assert.NoError(t, err)
	assert.Equal(t, "", result)

	// Just before suffix length
	hostname = "a-tsgateway"
	result, err = extractGatewayHostName(hostname)
	assert.NoError(t, err)
	assert.Equal(t, "a", result)
}

func TestIsGatewayHostName_EdgeCases(t *testing.T) {
	// Exact suffix
	assert.True(t, isGatewayHostName("-tsgateway"))

	// Almost suffix
	assert.False(t, isGatewayHostName("-tsgatewa"))
	assert.False(t, isGatewayHostName("-tsgatewayx"))
}
