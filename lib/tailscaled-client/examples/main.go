package main

import (
	"context"
	"fmt"
	"log"

	tailscaledclient "github.com/wold9168/k8s-cross-cluster/lib/tailscaled-client"
)

func main() {
	// Create a new Tailscale client
	client := tailscaledclient.New()

	// Get the current status of the Tailscale daemon
	ctx := context.Background()
	status, err := client.Status(ctx)
	if err != nil {
		log.Fatalf("Failed to get Tailscale status: %v", err)
	}

	// Print information about the current node
	fmt.Printf("=== Current Node ===\n")
	if status.Self != nil {
		fmt.Printf("Name: %s\n", status.Self.DNSName)
		fmt.Printf("ID: %s\n", status.Self.ID)
		fmt.Printf("Online: %t\n", status.Self.Online)
		fmt.Printf("IPs: %v\n\n", status.Self.TailscaleIPs)
	}

	// Get and print information about all peers
	fmt.Println("=== Peers ===")
	peers, err := client.Peers(ctx)
	if err != nil {
		log.Fatalf("Failed to get peers: %v", err)
	}

	for _, peer := range peers {
		// DNSName typically ends with ".", trim if needed
		dnsName := peer.DNSName
		if len(dnsName) > 0 && dnsName[len(dnsName)-1] == '.' {
			dnsName = dnsName[:len(dnsName)-1]
		}
		fmt.Printf("Domain: %-30s | HostName: %-15s | IPs: %v | Online: %t\n", dnsName, peer.HostName, peer.TailscaleIPs, peer.Online)
	}
}
