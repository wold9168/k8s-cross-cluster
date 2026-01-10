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

		// 获取当前命名空间中的所有 ConfigMap
		configMapList, err := GetAllConfigMapsInCurrentNamespace(clientset, nil)
		if err != nil {
			// 如果获取 ConfigMap 失败，记录错误但不 panic，继续执行
			klog.Errorf("Failed to list ConfigMaps: %v\n", err)
		} else {
			for _, cm := range configMapList.Items {
				klog.Infof("Successfully retrieved ConfigMap: %s\n", cm.Name)
			}
		}

		// 获取当前命名空间中的所有 Service
		serviceList, err := GetAllServicesInCurrentNamespace(clientset, nil)
		if err != nil {
			// 如果获取 Service 失败，记录错误但不 panic，继续执行
			klog.Errorf("Failed to list Services: %v\n", err)
		} else {
			for _, svc := range serviceList.Items {
				klog.Infof("Successfully retrieved Service: %s\n", svc.Name)
			}

			// 根据 Service 生成对应的跨集群访问域名
			remoteDomains, domainMapping := GenerateCrossClusterServiceDomains(serviceList)
			for _, remoteDomain := range remoteDomains {
				klog.Infof("Remote domain: %s -> Local domain: %s\n", remoteDomain, domainMapping[remoteDomain])
			}
		}

		// 根据跨集群访问域名生成对应的 ConfigMap

		// 将 ConfigMap 写入到集群中

		// 每次循环后暂停 10 秒，避免对 API Server 造成过大压力
		time.Sleep(10 * time.Second)
	}
}
