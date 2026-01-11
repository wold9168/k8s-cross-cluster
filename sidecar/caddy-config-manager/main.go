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
		if err := CheckPermissions(clientset, nil); err != nil {
			klog.Errorf("Permission check failed: %v, retrying in 10 seconds...", err)
			time.Sleep(10 * time.Second)
			continue
		}

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
			remoteDomains, domainMapping := GenerateCrossClusterServiceDomains(clientset, serviceList)
			for _, remoteDomain := range remoteDomains {
				klog.Infof("Remote domain: %s -> Local domain: %s\n", remoteDomain, domainMapping[remoteDomain])
			}

			// 根据跨集群访问域名生成对应的 ConfigMap
			caddyConfig := GenerateCaddyConfig(remoteDomains, domainMapping)

			// 将 ConfigMap 写入到集群中
			targetNamespace, nsErr := getCurrentNamespace()
			if nsErr != nil {
				klog.Warningf("Could not determine current namespace: %v", nsErr)
				targetNamespace = "unknown"
			}
			klog.Infof("Writing Caddy config to namespace '%s':\n%s", targetNamespace, caddyConfig)
			err = UpdateCaddyConfigMap(clientset, nil, caddyConfig)
			if err != nil {
				klog.Errorf("Failed to update Caddy ConfigMap: %v", err)
			}
		}

		// 每次循环后暂停 10 秒，避免对 API Server 造成过大压力
		time.Sleep(10 * time.Second)
	}
}
