package main

import (
	"context"
)

// PeerLister defines the interface for listing Tailscale peers
type PeerLister interface {
	GetPeers(ctx context.Context) ([]PeerInfo, error)
	GetSelf(ctx context.Context) (PeerInfo, error)
}

// DNSRecordStore defines the interface for storing DNS records
type DNSRecordStore interface {
	AddRecord(name string, recordType uint16, ttl uint32, value string)
	RemoveRecords(name string)
	GetRecordCount() int
	GetAllRecords() map[string][]DNSRecord
}

// CorefileUpdater defines the interface for updating Corefile configuration
type CorefileUpdater interface {
	EnsureConfig(ctx context.Context, upstreamServer string) error
	GetConfig() CoreDNSConfig
}

// CorefileValidator defines the interface for validating Corefile
type CorefileValidator interface {
	ValidateCorefile(content string) error
	NeedsUpdate(content, upstreamServer string) bool
	UpdateCorefile(content, upstreamServer string) string
}

// PermissionChecker defines the interface for checking Kubernetes permissions
type PermissionChecker interface {
	Check(ctx context.Context) error
}

// PeerDiscovery implements PeerLister
var (
	_ PeerLister = (*PeerDiscovery)(nil)
)

// CoreDNSUpdater implements CorefileUpdater
var (
	_ CorefileUpdater = (*CoreDNSUpdater)(nil)
)

// CorefileManager implements CorefileValidator
var (
	_ CorefileValidator = (*CorefileManager)(nil)
)
