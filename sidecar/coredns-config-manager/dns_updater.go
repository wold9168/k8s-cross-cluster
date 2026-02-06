package main

import (
	"fmt"
	"net"

	"github.com/miekg/dns"
	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
	"k8s.io/klog/v2"
)

// RemoteService 表示在远程集群中发现的服务
type RemoteService struct {
	Name      string
	Namespace string
	ClusterIP string
	Ports     []ServicePort
}

// ServicePort 表示服务暴露的端口
type ServicePort struct {
	Name     string
	Port     int32
	Protocol string
}

// UpdateDNSRecordsForGateways 为网关生成并添加 DNS 记录到 DNS 服务器
func UpdateDNSRecordsForGateways(dnsSrv *dnsserver.DNSServer) error {
	peers, err := getTailscalePeers()
	if err != nil {
		return fmt.Errorf("failed to get Tailscale peers: %w", err)
	}
	for _, peer := range peers {

		// 为每个发现的服务添加 DNS 记录
		// 格式: service.namespace.svc.clustername.remote
		nodename, err := extractGatewayHostNameFromPeerInfo(peer)
		if err != nil {
			return err
		}
		recordName := fmt.Sprintf("*.*.svc.%s.remote.", nodename)

		// 添加新记录之前删除此域的现有 DNS 记录
		dnsSrv.RemoveRecords(recordName)
		klog.Infof("Cleared existing DNS records for: %s", recordName)

		clusterIps := peer.TailscaleIPs
		klog.Infof("Fetch Addresses from PeerInfo. nodename, CurAddr: %s, %v", nodename, clusterIps)

		for _, clusterIp := range peer.TailscaleIPs {
			ip := net.ParseIP(clusterIp)
			if ip.To4() != nil {
				// 为服务添加 A 记录
				dnsSrv.AddRecord(recordName, dns.TypeA, 300 /* TTL */, clusterIp)
				klog.Infof("Added DNS A record: %s -> %s", recordName, clusterIp)
			} else if ip.To16() != nil {
				// 为服务添加 AAAA 记录
				dnsSrv.AddRecord(recordName, dns.TypeAAAA, 300 /* TTL */, clusterIp)
				klog.Infof("Added DNS AAAA record: %s -> %s", recordName, clusterIp)
			} else {
				return fmt.Errorf("Fatal error (coredns-config-manager): Invalid format of Address.")
			}
		}
	}
	return nil
}

// extractGatewayHostNames 提取以"-tsgateway"结尾的对等体主机名
func extractGatewayHostNameFromPeerInfo(peer PeerInfo) (string, error) {
	rawhostname := peer.HostName
	if len(rawhostname) >= 10 && rawhostname[len(rawhostname)-10:] == "-tsgateway" {
		return rawhostname[:len(rawhostname)-10], nil
	} else {
		return "", fmt.Errorf("Invalid format")
	}
}
