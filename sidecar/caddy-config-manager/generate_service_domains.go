package main

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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
	configMap, err := clientset.CoreV1().ConfigMaps("default").Get(context.TODO(), "tailscale-cluster-name", metav1.GetOptions{})
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
