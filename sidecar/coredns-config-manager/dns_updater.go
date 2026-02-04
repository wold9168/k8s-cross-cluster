package main

import (
	"fmt"
	"net"

	"github.com/miekg/dns"
	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
	"k8s.io/klog/v2"
)

// RemoteService represents a service discovered in a remote cluster
type RemoteService struct {
	Name      string
	Namespace string
	ClusterIP string
	Ports     []ServicePort
}

// ServicePort represents a port exposed by a service
type ServicePort struct {
	Name     string
	Port     int32
	Protocol string
}

// UpdateDNSRecordsForGateways generates and adds DNS records for the gateways to the DNS server
func UpdateDNSRecordsForGateways(dnsSrv *dnsserver.DNSServer) error {
	peers, err := getTailscalePeers()
	if err != nil {
		return fmt.Errorf("failed to get Tailscale peers: %w", err)
	}
	// Clear existing remote records before adding new ones
	// This prevents accumulation of stale records
	for _, peer := range peers {

		// Add DNS records for each discovered service
		// Format: service.namespace.svc.clustername.remote
		nodename, err := extractGatewayHostNameFromPeerInfo(peer)
		if err != nil {
			return err
		}
		recordName := fmt.Sprintf("*.*.svc.%s.remote.", nodename)

		// Remove existing DNS records for this domain before adding new ones
		dnsSrv.RemoveRecords(recordName)
		klog.Infof("Cleared existing DNS records for: %s", recordName)

		clusterIps := peer.TailscaleIPs
		klog.Infof("Fetch Addresses from PeerInfo. nodename, CurAddr: %s, %v", nodename, clusterIps)

		for _, clusterIp := range peer.TailscaleIPs {
			ip := net.ParseIP(clusterIp)
			if ip.To4() != nil {
				// Add A record for the service
				dnsSrv.AddRecord(recordName, dns.TypeA, 300 /* TTL */, clusterIp)
				klog.Infof("Added DNS A record: %s -> %s", recordName, clusterIp)
			} else if ip.To16() != nil {
				// Add AAAA record for the service
				dnsSrv.AddRecord(recordName, dns.TypeAAAA, 300 /* TTL */, clusterIp)
				klog.Infof("Added DNS AAAA record: %s -> %s", recordName, clusterIp)
			} else {
				return fmt.Errorf("Fatal error (coredns-config-manager): Invalid format of Address.")
			}
		}
	}
	return nil
}

// extractGatewayHostNames extracts hostnames of peers that end with "-tsgateway"
func extractGatewayHostNameFromPeerInfo(peer PeerInfo) (string, error) {
	rawhostname := peer.HostName
	if len(rawhostname) >= 10 && rawhostname[len(rawhostname)-10:] == "-tsgateway" {
		return rawhostname[:len(rawhostname)-10], nil
	} else {
		return "", fmt.Errorf("Invalid format")
	}
}
