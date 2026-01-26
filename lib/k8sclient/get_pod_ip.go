package k8sclient

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// GetCurrentPodIP retrieves the IP address of the current Pod
// It tries multiple sources in order of preference:
// 1. Kubernetes API (requires POD_NAME and POD_NAMESPACE)
// 2. Environment variable POD_IP
// 3. Network interfaces (fallback)
func GetCurrentPodIP(clientset kubernetes.Interface) (string, error) {
	var ip string
	var err error

	// Try 1: Get Pod IP from Kubernetes API
	ip, err = getPodIPFromK8sAPI(clientset)
	if err == nil && ip != "" {
		klog.Infof("Got Pod IP from K8s API: %s", ip)
		return ip, nil
	}
	klog.Infof("Failed to get Pod IP from K8s API: %v, trying fallback methods", err)

	// Try 2: Get Pod IP from environment variable
	ip = os.Getenv("POD_IP")
	if ip != "" {
		klog.Infof("Got Pod IP from POD_IP env: %s", ip)
		return ip, nil
	}

	// Try 3: Get IP from network interfaces
	ip, err = getLocalIP()
	if err == nil && ip != "" {
		klog.Infof("Got local IP from network interface: %s", ip)
		return ip, nil
	}

	return "", fmt.Errorf("failed to get Pod IP from all sources: %v", err)
}

// getPodIPFromK8sAPI retrieves the Pod IP using Kubernetes API
func getPodIPFromK8sAPI(clientset kubernetes.Interface) (string, error) {
	// Get Pod name from environment variable
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		// Try to get pod name from /proc/self/cgroup
		podName, _ = getPodNameFromCgroup()
		if podName == "" {
			return "", fmt.Errorf("POD_NAME environment variable not set and could not be derived")
		}
	}

	// Get namespace
	namespace, err := GetCurrentNamespace()
	if err != nil {
		return "", fmt.Errorf("failed to get namespace: %w", err)
	}

	// Get Pod object
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get pod %s/%s: %w", namespace, podName, err)
	}

	if pod.Status.PodIP == "" {
		return "", fmt.Errorf("pod IP is not assigned yet")
	}

	return pod.Status.PodIP, nil
}

// getPodNameFromCgroup derives the Pod name from /proc/self/cgroup
// This is useful when POD_NAME env var is not set
func getPodNameFromCgroup() (string, error) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		if strings.Contains(line, "kubepods") {
			// Expected format: .../kubepods/burstable/pod<id>/<containerid>
			// We need to extract the pod UID, then get pod name
			// This is complex, so for now we'll return empty
			// In real production, you might need to query the API server with the UID
			continue
		}
	}

	return "", fmt.Errorf("could not derive pod name from cgroup")
}

// getLocalIP retrieves the local IP address from network interfaces
// It prefers non-loopback, non-local interfaces
func getLocalIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Skip IPv6 and localhost
			if ip == nil || ip.IsLoopback() || ip.To4() == nil {
				continue
			}

			return ip.String(), nil
		}
	}

	return "", fmt.Errorf("no suitable IP address found")
}
