package generator

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	// Import the k8sclient package to use standardized ConfigMap operations
	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
)

// GenerateCrossClusterServiceDomains generates cross-cluster access domains for services
// Returns a slice of remote domains and a map from remote domain to local domain
func GenerateCrossClusterServiceDomains(clientset kubernetes.Interface, serviceList *v1.ServiceList) ([]string, map[string]string) {
	remoteDomains := make([]string, 0)
	domainMapping := make(map[string]string)

	if serviceList == nil {
		return remoteDomains, domainMapping
	}

	// Read cluster name from ConfigMap
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

		// Generate remote domain format: <service-name>.<namespace>.svc.<cluster-name>.remote
		remoteDomain := serviceName + "." + namespace + ".svc." + clusterName + ".remote"

		// Generate local domain format: <service-name>.<namespace>.svc.cluster.local
		localDomain := serviceName + "." + namespace + ".svc.cluster.local"

		// Add to slice
		remoteDomains = append(remoteDomains, remoteDomain)

		// Add to mapping
		domainMapping[remoteDomain] = localDomain
	}

	return remoteDomains, domainMapping
}
