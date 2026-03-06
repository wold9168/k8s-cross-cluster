package main

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"

	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
)

func TestDNSRecordManager_New(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	assert.NotNil(t, drm)
	assert.Equal(t, dnsSrv, drm.dnsServer)
}

func TestDNSRecordManager_UpdateRecordsForGateways(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	peers := []PeerInfo{
		{
			HostName:     "cluster1-tsgateway",
			TailscaleIPs: []string{"100.64.0.1"},
		},
		{
			HostName:     "cluster2-tsgateway",
			TailscaleIPs: []string{"100.64.0.2"},
		},
	}

	err := drm.UpdateRecordsForGateways(peers)

	assert.NoError(t, err)

	// Verify records were added
	records1 := dnsSrv.GetRecords("*.*.svc.cluster1.remote.")
	assert.Len(t, records1, 1)
	assert.Equal(t, "100.64.0.1", records1[0].Value)

	records2 := dnsSrv.GetRecords("*.*.svc.cluster2.remote.")
	assert.Len(t, records2, 1)
	assert.Equal(t, "100.64.0.2", records2[0].Value)
}

func TestDNSRecordManager_UpdateRecordsForGateways_InvalidHostname(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	peers := []PeerInfo{
		{
			HostName:     "invalid-hostname", // Not a gateway hostname
			TailscaleIPs: []string{"100.64.0.1"},
		},
	}

	err := drm.UpdateRecordsForGateways(peers)

	// Should skip invalid hostname without error
	assert.NoError(t, err)
}

func TestDNSRecordManager_UpdateRecordsForGateways_MultipleIPs(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	peers := []PeerInfo{
		{
			HostName:     "cluster1-tsgateway",
			TailscaleIPs: []string{"100.64.0.1", "fd7a:115c:a1e0::1"},
		},
	}

	err := drm.UpdateRecordsForGateways(peers)

	assert.NoError(t, err)

	// Should have both IPv4 and IPv6 records
	records := dnsSrv.GetRecords("*.*.svc.cluster1.remote.")
	assert.Len(t, records, 2)
}

func TestDNSRecordManager_UpdateRecordForSelf(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	self := PeerInfo{
		HostName:     "self-cluster-tsgateway",
		TailscaleIPs: []string{"100.64.0.100"},
	}

	err := drm.UpdateRecordForSelf(self)

	assert.NoError(t, err)

	records := dnsSrv.GetRecords("*.*.svc.self-cluster.remote.")
	assert.Len(t, records, 1)
	assert.Equal(t, "100.64.0.100", records[0].Value)
}

func TestDNSRecordManager_UpdateRecordForSelf_InvalidHostname(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	self := PeerInfo{
		HostName:     "invalid-hostname",
		TailscaleIPs: []string{"100.64.0.1"},
	}

	err := drm.UpdateRecordForSelf(self)

	assert.Error(t, err)
}

func TestDNSRecordManager_AddOrUpdateRecords_IPv4(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	err := drm.addOrUpdateRecords("test.remote.", "test-node", []string{"192.168.1.1"})

	assert.NoError(t, err)

	records := dnsSrv.GetRecords("test.remote.")
	assert.Len(t, records, 1)
	assert.Equal(t, dns.TypeA, records[0].Type)
	assert.Equal(t, "192.168.1.1", records[0].Value)
}

func TestDNSRecordManager_AddOrUpdateRecords_IPv6(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	err := drm.addOrUpdateRecords("test.remote.", "test-node", []string{"fd7a:115c:a1e0::1"})

	assert.NoError(t, err)

	records := dnsSrv.GetRecords("test.remote.")
	assert.Len(t, records, 1)
	assert.Equal(t, dns.TypeAAAA, records[0].Type)
	assert.Equal(t, "fd7a:115c:a1e0::1", records[0].Value)
}

func TestDNSRecordManager_AddOrUpdateRecords_MultipleIPs(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	ips := []string{"192.168.1.1", "192.168.1.2"}

	err := drm.addOrUpdateRecords("test.remote.", "test-node", ips)

	assert.NoError(t, err)

	records := dnsSrv.GetRecords("test.remote.")
	assert.Len(t, records, 2)
}

func TestDNSRecordManager_AddOrUpdateRecords_InvalidIP(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	err := drm.addOrUpdateRecords("test.remote.", "test-node", []string{"invalid-ip"})

	assert.NoError(t, err) // Should skip invalid IP without error

	records := dnsSrv.GetRecords("test.remote.")
	assert.Empty(t, records)
}

func TestDNSRecordManager_AddOrUpdateRecords_UpdateExisting(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	// Add initial record
	drm.addOrUpdateRecords("test.remote.", "test-node", []string{"192.168.1.1"})

	// Update with new IP
	drm.addOrUpdateRecords("test.remote.", "test-node", []string{"192.168.1.2"})

	records := dnsSrv.GetRecords("test.remote.")
	assert.Len(t, records, 1)
	assert.Equal(t, "192.168.1.2", records[0].Value)
}

func TestDNSRecordManager_GetRecordCount(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	// Initially empty
	assert.Equal(t, 0, drm.GetRecordCount())

	// Add some records
	drm.addOrUpdateRecords("test1.remote.", "node1", []string{"192.168.1.1"})
	drm.addOrUpdateRecords("test2.remote.", "node2", []string{"192.168.1.2"})

	assert.Equal(t, 2, drm.GetRecordCount())
}

func TestDNSRecordManager_GetAllRecords(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	drm.addOrUpdateRecords("test1.remote.", "node1", []string{"192.168.1.1"})
	drm.addOrUpdateRecords("test2.remote.", "node2", []string{"192.168.1.2"})

	allRecords := drm.GetAllRecords()

	assert.Len(t, allRecords, 2)
	assert.Contains(t, allRecords, "test1.remote.")
	assert.Contains(t, allRecords, "test2.remote.")
}

func TestDNSRecordManager_GetAllRecords_Empty(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:0")
	drm := NewDNSRecordManager(dnsSrv)

	allRecords := drm.GetAllRecords()

	assert.Empty(t, allRecords)
}

func TestDNSRecord_Convert(t *testing.T) {
	// Test DNSRecord type conversion
	dnsRecord := DNSRecord{
		Name:  "test.example.com",
		Type:  dns.TypeA,
		TTL:   300,
		Value: "192.168.1.1",
	}

	assert.Equal(t, "test.example.com", dnsRecord.Name)
	assert.Equal(t, dns.TypeA, dnsRecord.Type)
	assert.Equal(t, uint32(300), dnsRecord.TTL)
	assert.Equal(t, "192.168.1.1", dnsRecord.Value)
}
