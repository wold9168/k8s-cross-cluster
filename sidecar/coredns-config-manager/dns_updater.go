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
	} else {
		klog.Infof("get Tailscale peers successfully, cnt: %d", len(peers))
	}
	for _, peer := range peers {

		// 为每个发现的服务添加 DNS 记录
		// 格式: service.namespace.svc.clustername.remote
		nodename, err := extractGatewayHostNameFromPeerInfo(peer)
		if err != nil {
			continue
		}
		recordName := fmt.Sprintf("*.*.svc.%s.remote.", nodename)

		// 添加新记录之前删除此域的现有 DNS 记录
		err = addOrUpdateDNSRecords(dnsSrv, recordName, nodename, peer.TailscaleIPs)
		if err != nil {
			return fmt.Errorf("failed to update DNS records: %w", err)
		}
	}
	return nil
}

// addOrUpdateDNSRecords 为指定的记录名称添加或更新 DNS 记录
// 根据 IP 地址类型（IPv4/IPv6）添加相应的 A 或 AAAA 记录
func addOrUpdateDNSRecords(dnsSrv *dnsserver.DNSServer, recordName, nodename string, clusterIps []string) error {
	dnsSrv.RemoveRecords(recordName)

	klog.Infof("Cleared existing DNS records for: %s ; Fetch Addresses from PeerInfo. nodename, CurAddr: %s, %v",
		recordName, nodename, clusterIps)

	for _, clusterIp := range clusterIps {
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
	return nil
}

// extractGatewayHostNameFromPeerInfo 从 PeerInfo 中提取以"-tsgateway"结尾的 Peer 主机名
func extractGatewayHostNameFromPeerInfo(peer PeerInfo) (string, error) {
	rawhostname := peer.HostName
	return extractGatewayHostName(rawhostname)
}

// extractGatewayHostName 从以"-tsgateway"结尾的 Peer 主机名中抽取节点名
func extractGatewayHostName(rawhostname string) (string, error) {
	if len(rawhostname) >= 10 && rawhostname[len(rawhostname)-10:] == "-tsgateway" {
		return rawhostname[:len(rawhostname)-10], nil
	} else {
		return "", fmt.Errorf("Invalid format")
	}
}
