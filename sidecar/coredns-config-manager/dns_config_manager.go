package main

import (
	"context"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
	"github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/metrics"
	"github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/svc"
)

// DNSConfigManager orchestrates the complete DNS configuration lifecycle
type DNSConfigManager struct {
	clientset        *kubernetes.Clientset
	dnsServer        *dnsserver.DNSServer
	metricsManager   *metrics.Manager
	peerDiscovery    *PeerDiscovery
	dnsRecordManager *DNSRecordManager
	serviceDiscovery *ServiceDiscovery
	loadBalancer     *LoadBalancer
	coreDNSUpdater   *CoreDNSUpdater
	config           DNSConfigManagerConfig
}

// DNSConfigManagerConfig holds configuration for DNSConfigManager
type DNSConfigManagerConfig struct {
	SyncInterval  time.Duration
	SubDNSAddr    string
	MetricsAddr   string
	APIAddr       string
	CoreDNSConfig CoreDNSConfig
}

// DefaultDNSConfigManagerConfig returns default configuration
func DefaultDNSConfigManagerConfig() DNSConfigManagerConfig {
	return DNSConfigManagerConfig{
		SyncInterval: syncInterval,
		SubDNSAddr:   subdnsAddr,
		MetricsAddr:  metricsAddr,
		APIAddr:      svcAddr,
		CoreDNSConfig: CoreDNSConfig{
			Namespace:      CoreDNSNamespace,
			ConfigMapName:  CoreDNSConfigMapName,
			ConfigKey:      CoreDNSConfigKey,
			DeploymentName: CoreDNSDeploymentName,
			ManagedSection: ManagedSection{
				StartMarker: ManagedSectionStart,
				EndMarker:   ManagedSectionEnd,
			},
		},
	}
}

// NewDNSConfigManager creates a new DNSConfigManager
func NewDNSConfigManager(clientset *kubernetes.Clientset, config DNSConfigManagerConfig) *DNSConfigManager {
	dnsServer := dnsserver.NewDNSServer(config.SubDNSAddr)

	return &DNSConfigManager{
		clientset:        clientset,
		dnsServer:        dnsServer,
		metricsManager:   metrics.Init(),
		peerDiscovery:    NewPeerDiscovery(),
		dnsRecordManager: NewDNSRecordManager(dnsServer, clientset),
		coreDNSUpdater:   NewCoreDNSUpdater(clientset, config.CoreDNSConfig),
		config:           config,
	}
}

// Initialize initializes all components
func (dcm *DNSConfigManager) Initialize(ctx context.Context) error {
	// Initialize service discovery
	dcm.serviceDiscovery = NewServiceDiscovery(dcm.peerDiscovery)

	// Initialize load balancer
	dcm.loadBalancer = NewLoadBalancer(dcm.serviceDiscovery, dcm.dnsServer)

	// Register load balancer query handler with DNS server
	dcm.dnsServer.RegisterQueryHandler(dcm.loadBalancer.HandleQuery)

	// Start DNS server
	if err := dcm.dnsServer.Start(); err != nil {
		return err
	}

	// Start metrics server
	go func() {
		if err := dcm.metricsManager.Start(dcm.config.MetricsAddr); err != nil {
			klog.Errorf("Metrics server failed: %v", err)
		}
	}()

	// Start API server
	go func() {
		if err := svc.StartServer(dcm.config.APIAddr, dcm.dnsServer, dcm.clientset); err != nil {
			klog.Errorf("API server failed: %v", err)
		}
	}()

	// Initial service discovery
	if err := dcm.loadBalancer.RefreshServices(ctx); err != nil {
		klog.Errorf("Initial service discovery failed: %v", err)
	}

	klog.Info("DNSConfigManager initialized successfully")
	return nil
}

// Shutdown gracefully shuts down all components
func (dcm *DNSConfigManager) Shutdown() error {
	klog.Info("Shutting down DNSConfigManager")
	return dcm.dnsServer.Stop()
}

// Sync performs a complete synchronization cycle
func (dcm *DNSConfigManager) Sync(ctx context.Context) error {
	// Get Pod IP
	podIP, err := k8sclient.GetCurrentPodIP(dcm.clientset)
	if err != nil {
		return err
	}
	klog.Infof("Current Pod IP: %s", podIP)

	// Get Service ClusterIP
	currentSvcClusterIP, err := k8sclient.GetCurrentPodServiceClusterIP(dcm.clientset)
	if err != nil {
		klog.Warningf("Failed to get Service ClusterIP: %v", err)
		currentSvcClusterIP = ""
	} else {
		currentSvcClusterIP += subdnsPort
		klog.Infof("Current Service ClusterIP (with DNS port): %s", currentSvcClusterIP)
	}

	// Ensure CoreDNS configuration
	if err := dcm.coreDNSUpdater.EnsureConfig(ctx, currentSvcClusterIP); err != nil {
		return err
	}

	// Refresh service discovery from remote clusters
	if err := dcm.loadBalancer.RefreshServices(ctx); err != nil {
		klog.Errorf("Service discovery refresh failed: %v", err)
	}

	// Update DNS records from peers
	if err := dcm.updateDNSRecords(); err != nil {
		return err
	}

	// Update metrics
	recordCount := dcm.dnsRecordManager.GetRecordCount()
	serviceCount := dcm.loadBalancer.GetServiceCount()
	clusterCount := dcm.loadBalancer.GetClusterCount()

	dcm.metricsManager.UpdateDNSRecordCount(recordCount)
	klog.Infof("Updated metrics: DNS record count = %d, Services = %d, Clusters = %d",
		recordCount, serviceCount, clusterCount)

	return nil
}

// updateDNSRecords updates DNS records from Tailscale peers
func (dcm *DNSConfigManager) updateDNSRecords() error {
	ctx := context.Background()

	// Get gateway peers
	peers, err := dcm.peerDiscovery.GetPeers(ctx)
	if err != nil {
		return err
	}

	// Update records for all peers
	if err := dcm.dnsRecordManager.UpdateRecordsForGateways(peers); err != nil {
		return err
	}

	// Update record for self
	self, err := dcm.peerDiscovery.GetSelf(ctx)
	if err != nil {
		return err
	}

	return dcm.dnsRecordManager.UpdateRecordForSelf(self)
}

// Run starts the main synchronization loop
func (dcm *DNSConfigManager) Run(ctx context.Context) {
	ticker := time.NewTicker(dcm.config.SyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			klog.Info("Context cancelled, stopping DNSConfigManager")
			return
		case <-ticker.C:
			if err := dcm.Sync(ctx); err != nil {
				klog.Errorf("Sync failed: %v", err)
			}
		}
	}
}
