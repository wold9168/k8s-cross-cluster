package main

import (
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

func main() {
	// Authentication
	config, err := GetConfig()
	if err != nil {
		klog.Error("Authentication failed due to", err.Error())
		panic(err.Error())
	}
	// 使用上述配置创建一个 Kubernetes 客户端集（clientset），可用于访问所有 Kubernetes API 组
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Error("Creating clientset failed due to", err.Error())
		panic(err.Error())
	}

	for {
		// TODO: 添加事前的鉴权检查，应该直接检查当前鉴权上下文是否支持读写 ConfigMaps

		// 获取当前命名空间中的特定 ConfigMap
		configMapName := "example"
		_, err = GetSpecificConfigMapInCurrentNamespace(clientset, configMapName)
		if err != nil {
			// 如果获取 ConfigMap 失败，记录错误但不 panic，继续执行
			klog.Errorf("Failed to get ConfigMap %s: %v\n", configMapName, err)
		}

		// 获取当前集群指定命名空间的 Service

		// 根据 Service 生成对应的跨集群访问域名

		// 根据跨集群访问域名生成对应的 ConfigMap

		// 将 ConfigMap 写入到集群中

		// 每次循环后暂停 10 秒，避免对 API Server 造成过大压力
		time.Sleep(10 * time.Second)
	}
}
