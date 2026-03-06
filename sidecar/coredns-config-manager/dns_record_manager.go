package main

import (
	"fmt"
	"net"

	"github.com/miekg/dns"
	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
	"k8s.io/klog/v2"
)

// DNSRecordManager manages DNS records in the embedded DNS server
type DNSRecordManager struct {
	dnsServer *dnsserver.DNSServer
}

// NewDNSRecordManager creates a new DNSRecordManager
func NewDNSRecordManager(dnsServer *dnsserver.DNSServer) *DNSRecordManager {
	return &DNSRecordManager{
		dnsServer: dnsServer,
	}
}

// UpdateRecordsForGateways updates DNS records for all gateway peers
func (drm *DNSRecordManager) UpdateRecordsForGateways(peers []PeerInfo) error {
	for _, peer := range peers {
		nodeName, err := extractGatewayHostName(peer.HostName)
		if err != nil {
			klog.Warningf("Skipping peer %s: %v", peer.HostName, err)
			continue
		}

		recordName := fmt.Sprintf("*.*.svc.%s.remote.", nodeName)
		if err := drm.addOrUpdateRecords(recordName, nodeName, peer.TailscaleIPs); err != nil {
			klog.Errorf("Failed to update records for %s: %v", recordName, err)
		}
	}
	return nil
}

// UpdateRecordForSelf updates DNS record for the local node
func (drm *DNSRecordManager) UpdateRecordForSelf(self PeerInfo) error {
	nodeName, err := extractGatewayHostName(self.HostName)
	if err != nil {
		return fmt.Errorf("cannot extract node name from self: %w", err)
	}

	recordName := fmt.Sprintf("*.*.svc.%s.remote.", nodeName)
	return drm.addOrUpdateRecords(recordName, nodeName, self.TailscaleIPs)
}

// addOrUpdateRecords adds or updates DNS records for a given record name
func (drm *DNSRecordManager) addOrUpdateRecords(recordName, nodeName string, clusterIPs []string) error {
	// Remove existing records for this domain
	drm.dnsServer.RemoveRecords(recordName)

	klog.Infof("Cleared existing DNS records for: %s; Node: %s; IPs: %v",
		recordName, nodeName, clusterIPs)

	// Add new records based on IP type
	for _, clusterIP := range clusterIPs {
		ip := net.ParseIP(clusterIP)
		if ip == nil {
			klog.Warningf("Invalid IP address format: %s", clusterIP)
			continue
		}

		if ip.To4() != nil {
			// IPv4: Add A record
			drm.dnsServer.AddRecord(recordName, dns.TypeA, 300, clusterIP)
			klog.Infof("Added DNS A record: %s -> %s", recordName, clusterIP)
		} else if ip.To16() != nil {
			// IPv6: Add AAAA record
			drm.dnsServer.AddRecord(recordName, dns.TypeAAAA, 300, clusterIP)
			klog.Infof("Added DNS AAAA record: %s -> %s", recordName, clusterIP)
		} else {
			klog.Warningf("Unknown IP format: %s", clusterIP)
		}
	}

	return nil
}

// GetRecordCount returns the total number of DNS records
func (drm *DNSRecordManager) GetRecordCount() int {
	return drm.dnsServer.GetRecordCount()
}

// GetAllRecords returns all DNS records
func (drm *DNSRecordManager) GetAllRecords() map[string][]DNSRecord {
	rawRecords := drm.dnsServer.GetAllRecords()
	result := make(map[string][]DNSRecord)

	for name, records := range rawRecords {
		dnsRecords := make([]DNSRecord, len(records))
		for i, r := range records {
			dnsRecords[i] = DNSRecord{
				Name:  r.Name,
				Type:  r.Type,
				TTL:   r.TTL,
				Value: r.Value,
			}
		}
		result[name] = dnsRecords
	}

	return result
}
