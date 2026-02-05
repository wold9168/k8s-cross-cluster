package main

import (
	"context"
	"os"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
	"github.com/wold9168/k8s-cross-cluster/sidecar/caddy-config-manager/pkg/generator"
)

func CheckPermissions(clientset kubernetes.Interface, namespace *string) error {
	ctx := context.Background()
	ns := k8sclient.GetCurrentNamespaceOrProvided(namespace)

	klog.Infof("Checking permissions in namespace: %s", ns)

	// if err := k8sclient.CheckConfigMapPermissions(clientset, ctx, ns); err != nil {
	// 	return err
	// }

	if err := k8sclient.CheckServicePermissions(clientset, ctx, ns); err != nil {
		return err
	}

	klog.Infof("All required permissions verified in namespace: %s", ns)
	return nil
}

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
		// 鉴权检查：验证当前上下文是否支持读取 Services
		if err := CheckPermissions(clientset, nil); err != nil {
			klog.Errorf("Permission check failed: %v, retrying in 10 seconds...", err)
			time.Sleep(10 * time.Second)
			continue
		}


		// 获取当前命名空间中的所有 Service
		serviceList, err := k8sclient.GetAllServicesInCurrentNamespace(clientset, nil)
		if err != nil {
			// 如果获取 Service 失败，记录错误但不 panic，继续执行
			klog.Errorf("Failed to list Services: %v\n", err)
		} else {
			for _, svc := range serviceList.Items {
				klog.Infof("Successfully retrieved Service: %s\n", svc.Name)
			}

			// 根据 Service 生成对应的跨集群访问域名
			remoteDomains, domainMapping := generator.GenerateCrossClusterServiceDomains(clientset, serviceList)
			for _, remoteDomain := range remoteDomains {
				klog.Infof("Remote domain: %s -> Local domain: %s\n", remoteDomain, domainMapping[remoteDomain])
			}

			// 根据跨集群访问域名生成对应的 ConfigMap
			caddyConfig := generator.GenerateCaddyConfig(remoteDomains, domainMapping)

			// 将 ConfigMap 写入到容器的 /config/Caddyfile 文件中
			configDir := "/config"
			configPath := "/config/Caddyfile"

			// 确保目录存在
			if err := os.MkdirAll(configDir, 0755); err != nil {
				klog.Errorf("Failed to create config directory '%s': %v", configDir, err)
			} else {
				klog.Infof("Writing Caddy config to file '%s':\n%s", configPath, caddyConfig)
				if err := os.WriteFile(configPath, []byte(caddyConfig), 0644); err != nil {
					klog.Errorf("Failed to write Caddy config file '%s': %v", configPath, err)
				} else {
					klog.Infof("Successfully wrote Caddy config to file '%s'", configPath)
				}
			}
		}

		// 每次循环后暂停 10 秒，避免对 API Server 造成过大压力
		time.Sleep(10 * time.Second)
	}
}
