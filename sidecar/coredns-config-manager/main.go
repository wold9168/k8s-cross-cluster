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

	for {
		// 鉴权检查：验证当前上下文是否支持读写 ConfigMaps 和读取 Services
		if err := k8sclient.CheckPermissions(clientset, nil); err != nil {
			klog.Errorf("Permission check failed: %v, retrying in 10 seconds...", err)
			time.Sleep(10 * time.Second)
			continue
		}

		// 每次循环后暂停 10 秒，避免对 API Server 造成过大压力
		time.Sleep(10 * time.Second)
	}
}
