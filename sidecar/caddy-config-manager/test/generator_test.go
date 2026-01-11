package test

import (
	"testing"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/wold9168/k8s-cross-cluster/sidecar/caddy-config-manager/pkg/generator"
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
	remoteDomains, domainMapping := generator.GenerateCrossClusterServiceDomains(clientset, serviceList)

	if len(remoteDomains) != 2 {
		t.Errorf("Expected 2 remote domains, got: %d", len(remoteDomains))
	}

	if len(domainMapping) != 2 {
		t.Errorf("Expected 2 domain mappings, got: %d", len(domainMapping))
	}

	expectedRemote1 := "service1.test-ns.svc.foo.remote"
	expectedLocal1 := "service1.test-ns.svc.cluster.local"
	expectedRemote2 := "service2.test-ns.svc.foo.remote"
	expectedLocal2 := "service2.test-ns.svc.cluster.local"

	foundRemote1 := false
	foundRemote2 := false

	for _, domain := range remoteDomains {
		if domain == expectedRemote1 {
			foundRemote1 = true
		}
		if domain == expectedRemote2 {
			foundRemote2 = true
		}
	}

	if !foundRemote1 {
		t.Errorf("Expected to find remote domain: %s", expectedRemote1)
	}
	if !foundRemote2 {
		t.Errorf("Expected to find remote domain: %s", expectedRemote2)
	}

	if domainMapping[expectedRemote1] != expectedLocal1 {
		t.Errorf("Expected mapping %s -> %s, got: %s", expectedRemote1, expectedLocal1, domainMapping[expectedRemote1])
	}
	if domainMapping[expectedRemote2] != expectedLocal2 {
		t.Errorf("Expected mapping %s -> %s, got: %s", expectedRemote2, expectedLocal2, domainMapping[expectedRemote2])
	}
}

func TestGenerateCrossClusterServiceDomains_Nil(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	remoteDomains, domainMapping := generator.GenerateCrossClusterServiceDomains(clientset, nil)

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
	remoteDomains, domainMapping := generator.GenerateCrossClusterServiceDomains(clientset, serviceList)

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
	remoteDomains, domainMapping := generator.GenerateCrossClusterServiceDomains(clientset, serviceList)

	if len(remoteDomains) != 1 {
		t.Errorf("Expected 1 remote domain, got: %d", len(remoteDomains))
	}

	if len(domainMapping) != 1 {
		t.Errorf("Expected 1 domain mapping, got: %d", len(domainMapping))
	}

	expectedRemote := "service1.test-ns.svc.default-cluster-name.remote"
	expectedLocal := "service1.test-ns.svc.cluster.local"

	if len(remoteDomains) > 0 && remoteDomains[0] != expectedRemote {
		t.Errorf("Expected remote domain: %s, got: %s", expectedRemote, remoteDomains[0])
	}

	if domainMapping[expectedRemote] != expectedLocal {
		t.Errorf("Expected mapping %s -> %s, got: %s", expectedRemote, expectedLocal, domainMapping[expectedRemote])
	}
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

	config := generator.GenerateCaddyConfig(remoteDomains, domainMapping)

	expected := `service1.test-ns.svc.clusterwise.remote {
    reverse_proxy service1.test-ns.svc.cluster.local
}
service2.test-ns.svc.clusterwise.remote {
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

	config := generator.GenerateCaddyConfig(remoteDomains, domainMapping)

	if config != "" {
		t.Errorf("Expected empty config, got: %s", config)
	}
}

func TestGenerateCaddyConfig_MissingMapping(t *testing.T) {
	remoteDomains := []string{
		"service1.test-ns.svc.clusterwise.remote",
	}
	domainMapping := map[string]string{} // Empty mapping

	config := generator.GenerateCaddyConfig(remoteDomains, domainMapping)

	if config != "" {
		t.Errorf("Expected empty config due to missing mapping, got: %s", config)
	}
}