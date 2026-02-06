package main

import (
	"context"
	"fmt"

	tailscaledclient "github.com/wold9168/k8s-cross-cluster/lib/tailscaled-client"
	"k8s.io/klog/v2"
)

// PeerInfo 保存关于 Tailscale 对等体的信息
type PeerInfo struct {
	ID           string
	HostName     string
	DNSName      string
	TailscaleIPs []string
	Online       bool
}

// getTailscalePeers 获取当前节点的 Tailscale 对等节点
func getTailscalePeers() ([]PeerInfo, error) {
	client := tailscaledclient.New()
	ctx := context.Background()

	peers, err := client.Peers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Tailscale peers: %w", err)
	}

	peerInfos := make([]PeerInfo, 0, len(peers))
	klog.Infof("get %d Tailscale peers from tailscaled.sock", len(peers))
	for _, peer := range peers {
		// 清理 DNS 名称（如果存在则删除末尾的点）
		dnsName := peer.DNSName
		if len(dnsName) > 0 && dnsName[len(dnsName)-1] == '.' {
			dnsName = dnsName[:len(dnsName)-1]
		}

		// 将 Tailscale IP 转换为字符串
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
