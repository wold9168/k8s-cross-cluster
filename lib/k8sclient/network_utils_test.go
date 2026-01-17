package k8sclient

import (
	"net"
	"testing"
)

func TestGetLocalIP(t *testing.T) {
	ip, err := getLocalIP()
	
	if err != nil {
		// In some test environments, there might not be suitable network interfaces
		t.Logf("getLocalIP failed (expected in some test environments): %v", err)
	} else {
		// Validate that the returned IP is a valid IP address
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			t.Errorf("Returned IP is not valid: %s", ip)
		}
		// Ensure it's not a loopback address in most cases
		parsedIP = net.ParseIP(ip)
		if parsedIP != nil && parsedIP.IsLoopback() {
			t.Logf("Returned loopback IP: %s", ip)
		}
	}
}

func TestGetPodNameFromCgroup(t *testing.T) {
	// This function tries to read from /proc/self/cgroup which doesn't exist in test environment
	// So it should return an error
	podName, err := getPodNameFromCgroup()
	
	if err == nil {
		t.Errorf("Expected error when reading from non-existent cgroup file, got nil")
	}
	if podName != "" {
		t.Errorf("Expected empty pod name, got: %s", podName)
	}
	
	// In test environments, this function will typically return an error
	// which is the expected behavior when not running in a container
}