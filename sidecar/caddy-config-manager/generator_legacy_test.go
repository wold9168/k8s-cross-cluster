package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGenerateCrossClusterServiceDomains(t *testing.T) {
	namespace := "test-ns"
	serviceList := &v1.ServiceList{
		Items: []v1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: namespace,
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service2",
					Namespace: namespace,
				},
			},
		},
	}

	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tailscale-cluster-name",
				Namespace: "default",
			},
			Data: map[string]string{
				"CLUSTER_NAME": "foo",
			},
		},
	)
	remoteDomains, domainMapping := GenerateCrossClusterServiceDomains(clientset, serviceList)

	// Now generates 2 domain types per service: clusterset and cluster-specific
	if len(remoteDomains) != 4 {
		t.Errorf("Expected 4 remote domains, got: %d", len(remoteDomains))
	}

	if len(domainMapping) != 4 {
		t.Errorf("Expected 4 domain mappings, got: %d", len(domainMapping))
	}

	// Check clusterset domains
	assert.Contains(t, remoteDomains, "service1.test-ns.svc.clusterset.remote")
	assert.Contains(t, remoteDomains, "service2.test-ns.svc.clusterset.remote")

	// Check cluster-specific domains
	assert.Contains(t, remoteDomains, "service1.test-ns.svc.foo.remote")
	assert.Contains(t, remoteDomains, "service2.test-ns.svc.foo.remote")
}

func TestGenerateCrossClusterServiceDomains_Nil(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	remoteDomains, domainMapping := GenerateCrossClusterServiceDomains(clientset, nil)

	if len(remoteDomains) != 0 {
		t.Errorf("Expected 0 remote domains, got: %d", len(remoteDomains))
	}

	if len(domainMapping) != 0 {
		t.Errorf("Expected 0 domain mappings, got: %d", len(domainMapping))
	}
}

func TestGenerateCrossClusterServiceDomains_Empty(t *testing.T) {
	serviceList := &v1.ServiceList{
		Items: []v1.Service{},
	}

	clientset := fake.NewSimpleClientset()
	remoteDomains, domainMapping := GenerateCrossClusterServiceDomains(clientset, serviceList)

	if len(remoteDomains) != 0 {
		t.Errorf("Expected 0 remote domains, got: %d", len(remoteDomains))
	}

	if len(domainMapping) != 0 {
		t.Errorf("Expected 0 domain mappings, got: %d", len(domainMapping))
	}
}

func TestGenerateCrossClusterServiceDomains_EmptyClusterName(t *testing.T) {
	namespace := "test-ns"
	serviceList := &v1.ServiceList{
		Items: []v1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: namespace,
				},
			},
		},
	}

	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tailscale-cluster-name",
				Namespace: "default",
			},
			Data: map[string]string{
				"CLUSTER_NAME": "",
			},
		},
	)
	remoteDomains, domainMapping := GenerateCrossClusterServiceDomains(clientset, serviceList)

	// When cluster name is empty, generates clusterset and default cluster domains
	if len(remoteDomains) != 2 {
		t.Errorf("Expected 2 remote domains, got: %d", len(remoteDomains))
	}

	if len(domainMapping) != 2 {
		t.Errorf("Expected 2 domain mappings, got: %d", len(domainMapping))
	}

	// Check clusterset domain
	assert.Contains(t, remoteDomains, "service1.test-ns.svc.clusterset.remote")

	// Check default cluster domain
	assert.Contains(t, remoteDomains, "service1.test-ns.svc.default-cluster-name.remote")
	assert.Equal(t, "service1.test-ns.svc.cluster.local", domainMapping["service1.test-ns.svc.default-cluster-name.remote"])
	assert.Equal(t, "service1.test-ns.svc.cluster.local", domainMapping["service1.test-ns.svc.clusterset.remote"])
}

func TestGenerateCaddyConfig(t *testing.T) {
	remoteDomains := []string{
		"service1.test-ns.svc.clusterwise.remote",
		"service2.test-ns.svc.clusterwise.remote",
	}
	domainMapping := map[string]string{
		"service1.test-ns.svc.clusterwise.remote": "service1.test-ns.svc.cluster.local",
		"service2.test-ns.svc.clusterwise.remote": "service2.test-ns.svc.cluster.local",
	}

	config := GenerateCaddyConfig(remoteDomains, domainMapping)

	expected := `service1.test-ns.svc.clusterwise.remote {
    tls internal
    reverse_proxy service1.test-ns.svc.cluster.local
}
service2.test-ns.svc.clusterwise.remote {
    tls internal
    reverse_proxy service2.test-ns.svc.cluster.local
}
`

	if config != expected {
		t.Errorf("Expected config:\n%s\nGot:\n%s", expected, config)
	}
}

func TestGenerateCaddyConfig_Empty(t *testing.T) {
	remoteDomains := []string{}
	domainMapping := map[string]string{}

	config := GenerateCaddyConfig(remoteDomains, domainMapping)

	if config != "" {
		t.Errorf("Expected empty config, got: %s", config)
	}
}

func TestGenerateCaddyConfig_MissingMapping(t *testing.T) {
	remoteDomains := []string{
		"service1.test-ns.svc.clusterwise.remote",
	}
	domainMapping := map[string]string{} // Empty mapping

	config := GenerateCaddyConfig(remoteDomains, domainMapping)

	if config != "" {
		t.Errorf("Expected empty config due to missing mapping, got: %s", config)
	}
}
