package main

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
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
	ctx := context.Background()
	for {
		// 鉴权检查：验证当前上下文是否支持读写 ConfigMaps 和读取 Pods
		if ns, err := k8sclient.GetCurrentNamespace(); err != nil {
			klog.Error("Failed to get the current namespace: ", err)
		} else if err := k8sclient.CheckConfigMapPermissions(clientset, ctx, ns); err != nil {
			klog.Errorf("Permission check failed: %v, retrying in 10 seconds...", err)
			time.Sleep(10 * time.Second)
			continue
		} else if err := k8sclient.CheckPodPermissions(clientset, ctx, ns); err != nil {
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
		localhostIP := "127.0.0.1" // Loopback. The DNS SubServer is supported to be deployed in the same pod.
		if err := ensureCoreDNSConfig(clientset, localhostIP); err != nil {
			klog.Errorf("Failed to ensure CoreDNS configuration: %v", err)
		} else {
			klog.Info("CoreDNS configuration is properly set up")
		}

		// 获取当前节点的 Tailscale 对端节点，并根据 HostName 生成 *.*.svc.HostName.remote 这样的 DNS 记录，装入我们上面拉起来的 DNS 服务器里
		UpdateDNSRecordsForGateways(dnsSrv)

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
