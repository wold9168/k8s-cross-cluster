package main

import (
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
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

	// TODO: 在这里拉起来一个DNS服务器

	for {
		// 鉴权检查：验证当前上下文是否支持读写 ConfigMaps
		if ns, err := k8sclient.GetCurrentNamespace(); err != nil {
			klog.Errorf("Failed to get the current namespace: ", err)
		} else if err := k8sclient.CheckConfigMapPermissions(clientset, nil, ns); err != nil {
			klog.Errorf("Permission check failed: %v, retrying in 10 seconds...", err)
			time.Sleep(10 * time.Second)
			continue
		}

		// TODO: 获取上面拉起来的DNS服务器的IP
		// TODO: 检查Corefile(kube-system/configmaps/coredns)有没有设置我们拉起来的DNS服务器为上游。如果没有，则将对应的配置项置入
		//		记得为对应的配置项打注释，标注此为 coredns-configmap-manager 自动生成，以免引发困惑
		// TODO: 获取当前节点的 Tailscale 对端节点
		// TODO: 提取对端节点里以 -tsgateway 结尾的节点，提取他们的 HostName，HostName 就对应集群名
		// TODO: 根据 HostName 生成 *.*.svc.HostName.remote 这样的 DNS 记录，装入我们上面拉起来的 DNS 服务器里

		// 每次循环后暂停 10 秒，避免对 API Server 造成过大压力
		time.Sleep(10 * time.Second)
	}
}
