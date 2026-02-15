package main

import (
	"context"
	"fmt"

	tailscaledclient "github.com/wold9168/k8s-cross-cluster/lib/tailscaled-client"
	"k8s.io/klog/v2"
	"tailscale.com/ipn/ipnstate"
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
	ctx := context.TODO()

	peers, err := client.Peers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Tailscale peers: %w", err)
	}

	peerInfos := make([]PeerInfo, 0, len(peers))
	klog.Infof("get %d Tailscale peers from tailscaled.sock", len(peers))
	for _, peer := range peers {
		peerInfo, err := convertPeerToPeerInfo(peer)
		if err != nil {
			klog.Errorf("Failed to convert peer %s: %v", peer.HostName, err)
			continue
		}
		peerInfos = append(peerInfos, peerInfo)
	}

	return peerInfos, nil
}

// convertPeerToPeerInfo 将 ipnstate.PeerStatus 转换为 PeerInfo 格式
func convertPeerToPeerInfo(peer *ipnstate.PeerStatus) (PeerInfo, error) {
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
	return peerInfo, nil
}

// getCurrentTailscaleNode 获取当前节点的 Tailscale 节点
func getCurrentTailscaleNode() (PeerInfo, error) {
	client := tailscaledclient.New()
	ctx := context.TODO()

	self, err := client.Self(ctx)
	if err != nil {
		return PeerInfo{}, err
	}
	selfPeerInfo, err := convertPeerToPeerInfo(self)
	return selfPeerInfo, nil

}
