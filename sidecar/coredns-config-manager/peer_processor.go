package main

import (
	"context"
	"fmt"

	tailscaledclient "github.com/wold9168/k8s-cross-cluster/lib/tailscaled-client"
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
