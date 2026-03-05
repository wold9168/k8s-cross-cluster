package main

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
	"github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/metrics"
	"github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/svc"
)

func main() {
	// 初始化 metrics 管理器
	metricsManager := metrics.Init()

	config, err := k8sclient.GetConfig()
	if err != nil {
		klog.Error("Authentication failed due to ", err.Error())
		panic(err.Error())
	}
	// 使用上述配置创建一个 Kubernetes 客户端集（pt2clientset），可用于访问所有 Kubernetes API 组
	pt2clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("Creating clientset failed due to ", err.Error())
		panic(err.Error())
	}

	dnsSrv := dnsserver.NewDNSServer(subdnsAddr)
	if err := dnsSrv.Start(); err != nil {
		klog.Error("DNS server start failed: ", err.Error())
		panic(err.Error())
	}
	defer dnsSrv.Stop()

	// 启动 metrics HTTP 服务器
	go func() {
		if err := metricsManager.Start(metricsAddr); err != nil {
			klog.Errorf("Metrics server failed: %v", err)
		}
	}()

	// 启动 API HTTP 服务器
	go func() {
		if err := svc.StartServer(svcAddr, dnsSrv); err != nil {
			klog.Errorf("API server failed: %v", err)
		}
	}()

	ctx := context.TODO()

	for {
		err := authorization(pt2clientset, ctx)
		if err != nil {
			klog.Errorf("Authorization failed due to: %v", err)
			time.Sleep(syncInterval)
			continue
		}
		// 获取当前Pod的IP地址
		podIP, err := k8sclient.GetCurrentPodIP(pt2clientset)
		if err != nil {
			klog.Errorf("Failed to get Pod IP: %v, retrying in 10 seconds...", err)
			time.Sleep(syncInterval)
			continue
		}
		klog.Infof("Get Pod IP successfully, current Pod IP: %s", podIP)

		// 获取当前Pod所在服务的ClusterIP
		currentSvcClusterIp, err := k8sclient.GetCurrentPodServiceClusterIP(pt2clientset)
		if err != nil {
			currentSvcClusterIp = ""
			klog.Warningf("Failed to get Service ClusterIP: %v, using empty string", err)
		} else {
			// 硬编码端口号，该端口号对应 coredns-config-manager 子 dns 服务器的端口号
			currentSvcClusterIp += subdnsPort
			klog.Infof("Current Service ClusterIP (with dns port): %s", currentSvcClusterIp)
		}

		// 检查并更新 CoreDNS 配置以将 *.remote 查询转发到我们的 DNS 服务器
		if err := ensureCoreDNSConfig(pt2clientset, currentSvcClusterIp); err != nil {
			klog.ErrorS(err, "Failed to ensure CoreDNS configuration")
		} else {
			klog.Info("CoreDNS configuration is properly updated.")
		}

		// 获取当前节点的 Tailscale 对端节点，并根据 HostName 生成 *.*.svc.HostName.remote 这样的 DNS 记录，装入我们上面拉起来的 DNS 服务器里
		UpdateDNSRecordsForGateways(dnsSrv)
		if err != nil {
			klog.ErrorS(err, "Updating records of internal DNS server failed.")
		} else {
			klog.Info("Records of internal DNS server have been updated.")
			// 更新 DNS 记录数指标
			recordCount := dnsSrv.GetRecordCount()
			metricsManager.UpdateDNSRecordCount(recordCount)
			klog.Infof("Updated metrics: DNS record count = %d", recordCount)
		}

		// 每次循环后暂停 syncInterval 秒，避免对 API Server 造成过大压力
		time.Sleep(syncInterval)
	}
}

// authorization 验证当前上下文是否支持读写 ConfigMaps 和读取 Pods
func authorization(clientset *kubernetes.Clientset, ctx context.Context) error {
	if ns, err := k8sclient.GetCurrentNamespace(); err != nil {
		return fmt.Errorf("Failed to get the current namespace: %s", err)
	} else if err := k8sclient.CheckConfigMapPermissions(clientset, ctx, ns); err != nil {
		return fmt.Errorf("Permission check failed: %v, retrying in 10 seconds...", err)
	} else if err := k8sclient.CheckPodPermissions(clientset, ctx, ns); err != nil {
		return fmt.Errorf("Pod permission check failed: %v, retrying in 10 seconds...", err)
	}
	return nil
}

// ensureCoreDNSConfig 检查 CoreDNS 配置是否包含我们的上游配置
// 如有必要则进行更新
func ensureCoreDNSConfig(clientset *kubernetes.Clientset, upstreamServer string) error {
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

	ctx := context.TODO()
	for ; err != nil; err = k8sclient.RolloutDeployment(clientset, ctx, CoreDNSNamespace, CoreDNSDeploymentName) {
		klog.ErrorS(err, "failed to rollout CoreDNS: %w; retry will be performed in 10s")
		time.Sleep(10 * time.Second)
	}

	klog.Info("Successfully updated CoreDNS configuration to forward *.remote queries to ", upstreamServer)
	return nil
}
