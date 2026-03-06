package main

import (
	"context"

	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
	"k8s.io/klog/v2"
)

// getTailscalePeers retrieves Tailscale peer nodes
// Deprecated: Use PeerDiscovery.GetPeers instead
func getTailscalePeers() ([]PeerInfo, error) {
	pd := NewPeerDiscovery()
	ctx := context.Background()
	peers, err := pd.GetPeers(ctx)
	if err != nil {
		klog.Errorf("Failed to get Tailscale peers: %v", err)
		return nil, err
	}
	klog.Infof("Retrieved %d Tailscale peers", len(peers))
	return peers, nil
}

// getCurrentTailscaleNode retrieves the current Tailscale node
// Deprecated: Use PeerDiscovery.GetSelf instead
func getCurrentTailscaleNode() (PeerInfo, error) {
	pd := NewPeerDiscovery()
	ctx := context.Background()
	return pd.GetSelf(ctx)
}

// extractGatewayHostNameFromPeerInfo extracts gateway node name from peer
// Deprecated: Use PeerDiscovery.ExtractGatewayNodeName instead
func extractGatewayHostNameFromPeerInfo(peer PeerInfo) (string, error) {
	pd := NewPeerDiscovery()
	return pd.ExtractGatewayNodeName(peer)
}

// UpdateDNSRecordsForGateways updates DNS records for gateway peers
// Deprecated: Use DNSRecordManager.UpdateRecordsForGateways instead
func UpdateDNSRecordsForGateways(dnsSrv *dnsserver.DNSServer) error {
	pd := NewPeerDiscovery()
	drm := NewDNSRecordManager(dnsSrv)
	ctx := context.Background()

	peers, err := pd.GetPeers(ctx)
	if err != nil {
		return err
	}

	if err := drm.UpdateRecordsForGateways(peers); err != nil {
		return err
	}

	self, err := pd.GetSelf(ctx)
	if err != nil {
		return err
	}

	return drm.UpdateRecordForSelf(self)
}
