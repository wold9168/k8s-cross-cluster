package main

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
	tailscaledclient "github.com/wold9168/k8s-cross-cluster/lib/tailscaled-client"
	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
)

func main() {
	// Authentication
	config, err := k8sclient.GetConfig()
	if err != nil {
		klog.Error("Authentication failed due to ", err.Error())
		panic(err.Error())
	}
	// 使用上述配置创建一个 Kubernetes 客户端集（clientset），可用于访问所有 Kubernetes API 组
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("Creating clientset failed due to ", err.Error())
		panic(err.Error())
	}

	dnsSrv := dnsserver.NewDNSServer("0.0.0.0:10053")
	if err := dnsSrv.Start(); err != nil {
		klog.Error("DNS server start failed: ", err.Error())
		panic(err.Error())
	}
	defer dnsSrv.Stop()

	for {
		// 鉴权检查：验证当前上下文是否支持读写 ConfigMaps 和读取 Pods
		if ns, err := k8sclient.GetCurrentNamespace(); err != nil {
			klog.Error("Failed to get the current namespace: ", err)
		} else if err := k8sclient.CheckConfigMapPermissions(clientset, nil, ns); err != nil {
			klog.Errorf("Permission check failed: %v, retrying in 10 seconds...", err)
			time.Sleep(10 * time.Second)
			continue
		} else if err := k8sclient.CheckPodPermissions(clientset, nil, ns); err != nil {
			klog.Errorf("Pod permission check failed: %v, retrying in 10 seconds...", err)
			time.Sleep(10 * time.Second)
			continue
		}
		klog.Info("Authorization check passed.")

		// 获取当前Pod的IP地址
		podIP, err := k8sclient.GetCurrentPodIP(clientset)
		if err != nil {
			klog.Errorf("Failed to get Pod IP: %v, retrying in 10 seconds...", err)
			time.Sleep(10 * time.Second)
			continue
		}
		klog.Infof("Current Pod IP: %s", podIP)

		// Check and update CoreDNS configuration to forward *.remote queries to our DNS server
		if err := ensureCoreDNSConfig(clientset, dnsSrv.GetAddr()); err != nil {
			klog.Errorf("Failed to ensure CoreDNS configuration: %v", err)
		} else {
			klog.Info("CoreDNS configuration is properly set up")
		}

		// 获取当前节点的 Tailscale 对端节点
		peers, err := getTailscalePeers()
		if err != nil {
			klog.Errorf("Failed to get Tailscale peers: %v", err)
		} else {
			klog.Infof("Retrieved %d Tailscale peers", len(peers))

			// 提取对端节点里以 -tsgateway 结尾的节点，提取他们的 HostName，HostName 就对应集群名
			gatewayHostNames := extractGatewayHostNames(peers)
			klog.Infof("Found %d gateway nodes", len(gatewayHostNames))

			// 根据 HostName 生成 *.*.svc.HostName.remote 这样的 DNS 记录，装入我们上面拉起来的 DNS 服务器里
			updateDNSRecordsForGateways(dnsSrv, gatewayHostNames)

			// 打印对端节点信息
			for _, peer := range peers {
				klog.V(4).Infof("Peer: ID=%s, HostName=%s, DNSName=%s, IPs=%v, Online=%t",
					peer.ID, peer.HostName, peer.DNSName, peer.TailscaleIPs, peer.Online)
			}
		}

		// 每次循环后暂停 10 秒，避免对 API Server 造成过大压力
		time.Sleep(10 * time.Second)
	}
}

// ensureCoreDNSConfig checks if the CoreDNS configuration contains our upstream configuration
// and updates it if necessary
func ensureCoreDNSConfig(clientset kubernetes.Interface, upstreamServer string) error {
	// Get the current CoreDNS ConfigMap
	namespace := CoreDNSNamespace
	coreDNSCM, err := k8sclient.GetConfigMap(clientset, &namespace, CoreDNSConfigMapName)
	if err != nil {
		return fmt.Errorf("failed to get CoreDNS ConfigMap: %w", err)
	}

	// Get the current Corefile content
	currentCorefile, exists := coreDNSCM.Data[CoreDNSConfigKey]
	if !exists {
		return fmt.Errorf("Corefile key '%s' does not exist in ConfigMap", CoreDNSConfigKey)
	}

	// Check if update is needed
	if !needsUpdate(currentCorefile, upstreamServer) {
		klog.V(4).Info("CoreDNS configuration is already up to date")
		return nil
	}

	// Update the Corefile content
	updatedCorefile := updateCorefile(currentCorefile, upstreamServer)

	// Update the ConfigMap with the new Corefile
	coreDNSCM.Data[CoreDNSConfigKey] = updatedCorefile
	namespace = CoreDNSNamespace
	_, err = k8sclient.UpdateExistingConfigMap(clientset, &namespace, coreDNSCM)
	if err != nil {
		return fmt.Errorf("failed to update CoreDNS ConfigMap: %w", err)
	}

	klog.Info("Successfully updated CoreDNS configuration to forward *.remote queries to ", upstreamServer)
	return nil
}

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

// updateDNSRecordsForGateways generates and adds DNS records for the gateways to the DNS server
func updateDNSRecordsForGateways(dnsSrv *dnsserver.DNSServer, gatewayHostNames []string) {
	// Clear existing remote records before adding new ones
	// This prevents accumulation of stale records
	for _, gatewayName := range gatewayHostNames {
		// Discover services from the remote cluster
		remoteServices := discoverRemoteClusterServices(gatewayName)

		// Add DNS records for each discovered service
		for _, svc := range remoteServices {
			// Format: service.namespace.svc.clustername.remote
			recordName := fmt.Sprintf("%s.%s.svc.%s.remote.", svc.Name, svc.Namespace, gatewayName)

			// Add A record for the service
			dnsSrv.AddRecord(recordName, 1 /* dns.TypeA */, 300 /* TTL */, svc.ClusterIP)
			klog.Infof("Added DNS A record: %s -> %s", recordName, svc.ClusterIP)

			// Add SRV record for the service if it has ports
			for _, port := range svc.Ports {
				srvRecordName := fmt.Sprintf("_%s._%s.%s.%s.svc.%s.remote.",
					port.Name, port.Protocol, svc.Name, svc.Namespace, gatewayName)

				// Format for SRV record: priority weight port target
				srvTarget := fmt.Sprintf("%s.%s.svc.%s.remote.", svc.Name, svc.Namespace, gatewayName)
				srvData := fmt.Sprintf("0 50 %d %s", port.Port, srvTarget)

				dnsSrv.AddRecord(srvRecordName, 33 /* dns.TypeSRV */, 300 /* TTL */, srvData)
				klog.Infof("Added DNS SRV record: %s -> %s", srvRecordName, srvData)
			}
		}
	}
}

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

// discoverRemoteClusterServices discovers services in the remote cluster
// In a real implementation, this would connect to the remote cluster and fetch services
// For now, we'll simulate discovery with mock data
func discoverRemoteClusterServices(clusterName string) []RemoteService {
	// In a real implementation, this function would:
	// 1. Establish connection to the remote cluster using the clusterName
	// 2. Query the remote cluster's API server for services
	// 3. Return the discovered services

	// For demonstration purposes, returning mock services
	// In reality, you would implement actual service discovery here
	mockServices := []RemoteService{
		{
			Name:      "nginx-service",
			Namespace: "default",
			ClusterIP: "10.96.10.10",
			Ports: []ServicePort{
				{
					Name:     "http",
					Port:     80,
					Protocol: "TCP",
				},
			},
		},
		{
			Name:      "database-service",
			Namespace: "backend",
			ClusterIP: "10.96.10.20",
			Ports: []ServicePort{
				{
					Name:     "postgres",
					Port:     5432,
					Protocol: "TCP",
				},
			},
		},
	}

	klog.V(4).Infof("Discovered %d services in cluster %s", len(mockServices), clusterName)
	return mockServices
}
