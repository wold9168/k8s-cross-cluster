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
		klog.Infof("Get Pod IP successfully, current Pod IP: %s", podIP)

		// 获取当前Pod所在服务的ClusterIP
		currentSvcClusterIp, err := k8sclient.GetCurrentPodServiceClusterIP(clientset)
		if err != nil {
			klog.Warningf("Failed to get Service ClusterIP: %v, using empty string", err)
			currentSvcClusterIp = ""
		} else {
			// 硬编码端口号，该端口号对应 coredns-config-manager 子 dns 服务器的端口号
			currentSvcClusterIp += ":10053"
		}
		klog.Infof("Current Service ClusterIP (with dns port): %s", currentSvcClusterIp)

		// 检查并更新 CoreDNS 配置以将 *.remote 查询转发到我们的 DNS 服务器
		if err := ensureCoreDNSConfig(clientset, currentSvcClusterIp); err != nil {
			klog.Errorf("Failed to ensure CoreDNS configuration: %v", err)
		} else {
			klog.Info("CoreDNS configuration is properly updated. Rollout manually is needed.")
		}

		// 获取当前节点的 Tailscale 对端节点，并根据 HostName 生成 *.*.svc.HostName.remote 这样的 DNS 记录，装入我们上面拉起来的 DNS 服务器里
		UpdateDNSRecordsForGateways(dnsSrv)
		if err != nil {
			klog.Info("Records of internal DNS server have been updated.")
		} else {
			klog.Infof("Updating records of internal DNS server failed. %w", err)
		}

		// 每次循环后暂停 10 秒，避免对 API Server 造成过大压力
		time.Sleep(10 * time.Second)
	}
}

// ensureCoreDNSConfig 检查 CoreDNS 配置是否包含我们的上游配置
// 如有必要则进行更新
func ensureCoreDNSConfig(clientset kubernetes.Interface, upstreamServer string) error {
	// 获取当前 CoreDNS ConfigMap
	namespace := CoreDNSNamespace
	coreDNSCM, err := k8sclient.GetConfigMap(clientset, &namespace, CoreDNSConfigMapName)
	if err != nil {
		return fmt.Errorf("failed to get CoreDNS ConfigMap: %w", err)
	}

	// 获取当前 Corefile 内容
	currentCorefile, exists := coreDNSCM.Data[CoreDNSConfigKey]
	if !exists {
		return fmt.Errorf("Corefile key '%s' does not exist in ConfigMap", CoreDNSConfigKey)
	}

	// 检查是否需要更新
	if !needsUpdate(currentCorefile, upstreamServer) {
		klog.V(4).Info("CoreDNS configuration is already up to date")
		return nil
	}

	// 更新 Corefile 内容
	updatedCorefile := updateCorefile(currentCorefile, upstreamServer)

	// 使用新的 Corefile 更新 ConfigMap
	coreDNSCM.Data[CoreDNSConfigKey] = updatedCorefile
	namespace = CoreDNSNamespace
	_, err = k8sclient.UpdateExistingConfigMap(clientset, &namespace, coreDNSCM)
	if err != nil {
		return fmt.Errorf("failed to update CoreDNS ConfigMap: %w", err)
	}

	ctx := context.Background()
	for ; err != nil; err = k8sclient.RolloutDeployment(clientset, ctx, CoreDNSNamespace, CoreDNSDeploymentName) {
		time.Sleep(10 * time.Second)
		return fmt.Errorf("failed to rollout CoreDNS: %w", err)
	}

	klog.Info("Successfully updated CoreDNS configuration to forward *.remote queries to ", upstreamServer)
	return nil
}
