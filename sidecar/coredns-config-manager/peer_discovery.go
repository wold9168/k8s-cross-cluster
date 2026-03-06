package main

import (
	"context"
	"fmt"

	tailscaledclient "github.com/wold9168/k8s-cross-cluster/lib/tailscaled-client"
	"k8s.io/klog/v2"
	"tailscale.com/ipn/ipnstate"
)

// PeerDiscovery discovers Tailscale peer nodes
type PeerDiscovery struct {
	client *tailscaledclient.Client
}

// NewPeerDiscovery creates a new PeerDiscovery instance
func NewPeerDiscovery() *PeerDiscovery {
	return &PeerDiscovery{
		client: tailscaledclient.New(),
	}
}

// GetPeers retrieves all Tailscale peer nodes
func (pd *PeerDiscovery) GetPeers(ctx context.Context) ([]PeerInfo, error) {
	peers, err := pd.client.Peers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Tailscale peers: %w", err)
	}

	peerInfos := make([]PeerInfo, 0, len(peers))
	for _, peer := range peers {
		peerInfo, err := pd.convertPeerToPeerInfo(peer)
		if err != nil {
			klog.Errorf("Failed to convert peer %s: %v", peer.HostName, err)
			continue
		}
		peerInfos = append(peerInfos, peerInfo)
	}

	klog.Infof("Retrieved %d Tailscale peers", len(peerInfos))
	return peerInfos, nil
}

// GetSelf retrieves the current Tailscale node information
func (pd *PeerDiscovery) GetSelf(ctx context.Context) (PeerInfo, error) {
	self, err := pd.client.Self(ctx)
	if err != nil {
		return PeerInfo{}, fmt.Errorf("failed to get self node info: %w", err)
	}
	return pd.convertPeerToPeerInfo(self)
}

// convertPeerToPeerInfo converts ipnstate.PeerStatus to PeerInfo
func (pd *PeerDiscovery) convertPeerToPeerInfo(peer *ipnstate.PeerStatus) (PeerInfo, error) {
	// Clean DNS name (remove trailing dot)
	dnsName := peer.DNSName
	if len(dnsName) > 0 && dnsName[len(dnsName)-1] == '.' {
		dnsName = dnsName[:len(dnsName)-1]
	}

	// Convert Tailscale IPs to strings
	ipStrings := make([]string, len(peer.TailscaleIPs))
	for i, ip := range peer.TailscaleIPs {
		ipStrings[i] = ip.String()
	}

	return PeerInfo{
		ID:           string(peer.ID),
		HostName:     peer.HostName,
		DNSName:      dnsName,
		TailscaleIPs: ipStrings,
		Online:       peer.Online,
	}, nil
}

// GetGatewayPeers retrieves only gateway peers (hostnames ending with "-tsgateway")
func (pd *PeerDiscovery) GetGatewayPeers(ctx context.Context) ([]PeerInfo, error) {
	allPeers, err := pd.GetPeers(ctx)
	if err != nil {
		return nil, err
	}

	gatewayPeers := make([]PeerInfo, 0)
	for _, peer := range allPeers {
		if isGatewayHostName(peer.HostName) {
			gatewayPeers = append(gatewayPeers, peer)
		}
	}

	return gatewayPeers, nil
}

// ExtractGatewayNodeName extracts node name from gateway hostname
func (pd *PeerDiscovery) ExtractGatewayNodeName(peer PeerInfo) (string, error) {
	return extractGatewayHostName(peer.HostName)
}

// extractGatewayHostName extracts node name from hostname ending with "-tsgateway"
func extractGatewayHostName(rawHostName string) (string, error) {
	const gatewaySuffix = "-tsgateway"
	if len(rawHostName) >= len(gatewaySuffix) && rawHostName[len(rawHostName)-len(gatewaySuffix):] == gatewaySuffix {
		return rawHostName[:len(rawHostName)-len(gatewaySuffix)], nil
	}
	return "", fmt.Errorf("hostname does not end with %q: %s", gatewaySuffix, rawHostName)
}

// isGatewayHostName checks if hostname indicates a gateway node
func isGatewayHostName(hostName string) bool {
	_, err := extractGatewayHostName(hostName)
	return err == nil
}
