package generator

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	// 导入 k8sclient 包以使用标准化的 ConfigMap 操作
	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
)

// GenerateCrossClusterServiceDomains 为服务生成跨集群访问域名
// 返回远程域名切片和从远程域名到本地域名的映射
func GenerateCrossClusterServiceDomains(clientset kubernetes.Interface, serviceList *v1.ServiceList) ([]string, map[string]string) {
	remoteDomains := make([]string, 0)
	domainMapping := make(map[string]string)

	if serviceList == nil {
		return remoteDomains, domainMapping
	}

	// 从 ConfigMap 读取集群名称
	clusterName := "default-cluster-name" // Default fallback value
	defaultNs := "default"
	configMap, err := k8sclient.GetConfigMap(clientset, &defaultNs, "tailscale-cluster-name")
	if err != nil {
		klog.Warningf("Failed to get tailscale-cluster-name ConfigMap: %v, using default cluster name '%s'", err, clusterName)
	} else {
		if name, exists := configMap.Data["CLUSTER_NAME"]; exists && name != "" {
			clusterName = name
			klog.Infof("Using cluster name from tailscale-cluster-name ConfigMap: CLUSTER_NAME = %s", clusterName)
		} else {
			klog.Warningf("CLUSTER_NAME not found or empty in ConfigMap, using default '%s' to generate caddy's configuration", clusterName)
		}
	}

	for _, service := range serviceList.Items {
		serviceName := service.Name
		namespace := service.Namespace

		// 生成远程域名格式: <service-name>.<namespace>.svc.<cluster-name>.remote
		remoteDomain := serviceName + "." + namespace + ".svc." + clusterName + ".remote"

		// 生成本地域名格式: <service-name>.<namespace>.svc.cluster.local
		localDomain := serviceName + "." + namespace + ".svc.cluster.local"

		// 添加到切片
		remoteDomains = append(remoteDomains, remoteDomain)

		// 添加到映射
		domainMapping[remoteDomain] = localDomain
	}

	return remoteDomains, domainMapping
}
