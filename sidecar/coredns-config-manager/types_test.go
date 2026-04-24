package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPeerInfo(t *testing.T) {
	peer := PeerInfo{
		ID:           "test-id",
		HostName:     "test-host",
		DNSName:      "test.example.com",
		TailscaleIPs: []string{"100.64.0.1", "fd7a:115c:a1e0::1"},
		Online:       true,
	}

	assert.Equal(t, "test-id", peer.ID)
	assert.Equal(t, "test-host", peer.HostName)
	assert.Equal(t, "test.example.com", peer.DNSName)
	assert.Len(t, peer.TailscaleIPs, 2)
	assert.True(t, peer.Online)
}

func TestRemoteService(t *testing.T) {
	svc := RemoteService{
		Name:      "test-svc",
		Namespace: "test-ns",
		ClusterIP: "10.0.0.1",
		Ports: []ServicePort{
			{Name: "http", Port: 80, Protocol: "TCP"},
			{Name: "https", Port: 443, Protocol: "TCP"},
		},
	}

	assert.Equal(t, "test-svc", svc.Name)
	assert.Equal(t, "test-ns", svc.Namespace)
	assert.Equal(t, "10.0.0.1", svc.ClusterIP)
	assert.Len(t, svc.Ports, 2)
}

func TestServicePort(t *testing.T) {
	port := ServicePort{
		Name:     "http",
		Port:     8080,
		Protocol: "TCP",
	}

	assert.Equal(t, "http", port.Name)
	assert.Equal(t, int32(8080), port.Port)
	assert.Equal(t, "TCP", port.Protocol)
}

func TestDNSRecord(t *testing.T) {
	record := DNSRecord{
		Name:  "test.example.com",
		Type:  1, // A record
		TTL:   300,
		Value: "192.168.1.1",
	}

	assert.Equal(t, "test.example.com", record.Name)
	assert.Equal(t, uint16(1), record.Type)
	assert.Equal(t, uint32(300), record.TTL)
	assert.Equal(t, "192.168.1.1", record.Value)
}

func TestManagedSection(t *testing.T) {
	section := ManagedSection{
		StartMarker: "### START MARKER ###",
		EndMarker:   "### END MARKER ###",
	}

	assert.Equal(t, "### START MARKER ###", section.StartMarker)
	assert.Equal(t, "### END MARKER ###", section.EndMarker)
}

func TestCoreDNSConfigStruct(t *testing.T) {
	config := CoreDNSConfig{
		Namespace:      "test-ns",
		ConfigMapName:  "test-cm",
		ConfigKey:      "test-key",
		DeploymentName: "test-deploy",
		ManagedSection: ManagedSection{
			StartMarker: "### START ###",
			EndMarker:   "### END ###",
		},
	}

	assert.Equal(t, "test-ns", config.Namespace)
	assert.Equal(t, "test-cm", config.ConfigMapName)
	assert.Equal(t, "test-deploy", config.DeploymentName)
	assert.Equal(t, "### START ###", config.ManagedSection.StartMarker)
	assert.Equal(t, "### END ###", config.ManagedSection.EndMarker)
}
