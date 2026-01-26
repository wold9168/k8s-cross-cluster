package main

import (
	"context"
	"fmt"

	tailscaledclient "github.com/wold9168/k8s-cross-cluster/lib/tailscaled-client"
	"k8s.io/klog/v2"
)

// PeerInfo holds information about a Tailscale peer
type PeerInfo struct {
	ID           string
	HostName     string
	DNSName      string
	TailscaleIPs []string
	Online       bool
}

// getTailscalePeers retrieves the current node's Tailscale peer nodes
func getTailscalePeers() ([]PeerInfo, error) {
	client := tailscaledclient.New()
	ctx := context.Background()

	peers, err := client.Peers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Tailscale peers: %w", err)
	}

	peerInfos := make([]PeerInfo, 0, len(peers))
	for _, peer := range peers {
		// Clean up DNS name (remove trailing dot if present)
		dnsName := peer.DNSName
		if len(dnsName) > 0 && dnsName[len(dnsName)-1] == '.' {
			dnsName = dnsName[:len(dnsName)-1]
		}

		// Convert Tailscale IPs to strings
		ipStrings := make([]string, len(peer.TailscaleIPs))
		for i, ip := range peer.TailscaleIPs {
			ipStrings[i] = ip.String()
		}

		peerInfo := PeerInfo{
			ID:           string(peer.ID),
			HostName:     peer.HostName,
			DNSName:      dnsName,
			TailscaleIPs: ipStrings,
			Online:       peer.Online,
		}
		peerInfos = append(peerInfos, peerInfo)
	}

	return peerInfos, nil
}

// extractGatewayHostNames extracts hostnames of peers that end with "-tsgateway"
func extractGatewayHostNames(peers []PeerInfo) []string {
	var gatewayHostNames []string

	for _, peer := range peers {
		if len(peer.HostName) >= 10 && peer.HostName[len(peer.HostName)-10:] == "-tsgateway" {
			gatewayHostNames = append(gatewayHostNames, peer.HostName)
		}
	}

	return gatewayHostNames
}

// GetGatewayHostNamesFromPeers retrieves Tailscale peers and extracts gateway hostnames
func GetGatewayHostNamesFromPeers() ([]string, error) {
	peers, err := getTailscalePeers()
	if err != nil {
		return nil, fmt.Errorf("failed to get Tailscale peers: %w", err)
	}

	klog.Infof("Retrieved %d Tailscale peers", len(peers))

	// Print peer information for debugging
	for _, peer := range peers {
		klog.V(4).Infof("Peer: ID=%s, HostName=%s, DNSName=%s, IPs=%v, Online=%t",
			peer.ID, peer.HostName, peer.DNSName, peer.TailscaleIPs, peer.Online)
	}

	gatewayHostNames := extractGatewayHostNames(peers)
	klog.Infof("Found %d gateway nodes", len(gatewayHostNames))

	return gatewayHostNames, nil
}
