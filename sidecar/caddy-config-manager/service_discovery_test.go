package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestServiceDiscovery_New(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	sd := NewServiceDiscovery(clientset)

	assert.NotNil(t, sd)
	assert.Equal(t, "default-cluster-name", sd.clusterName)
	assert.Equal(t, "default", sd.namespace)
}

func TestServiceDiscovery_WithOptions(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	sd := NewServiceDiscovery(
		clientset,
		WithClusterName("test-cluster"),
		WithNamespace("test-ns"),
	)

	assert.Equal(t, "test-cluster", sd.clusterName)
	assert.Equal(t, "test-ns", sd.namespace)
}

func TestServiceDiscovery_LoadClusterNameFromConfigMap(t *testing.T) {
	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tailscale-cluster-name",
				Namespace: "default",
			},
			Data: map[string]string{
				"CLUSTER_NAME": "test-cluster-from-cm",
			},
		},
	)

	sd := NewServiceDiscovery(clientset)
	err := sd.LoadClusterNameFromConfigMap("tailscale-cluster-name")

	assert.NoError(t, err)
	assert.Equal(t, "test-cluster-from-cm", sd.GetClusterName())
}

func TestServiceDiscovery_LoadClusterNameFromConfigMap_NotFound(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	sd := NewServiceDiscovery(clientset)
	err := sd.LoadClusterNameFromConfigMap("nonexistent")

	assert.NoError(t, err) // Should not fail, just use default
	assert.Equal(t, "default-cluster-name", sd.GetClusterName())
}

func TestServiceDiscovery_LoadClusterNameFromConfigMap_EmptyValue(t *testing.T) {
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

	sd := NewServiceDiscovery(clientset)
	err := sd.LoadClusterNameFromConfigMap("tailscale-cluster-name")

	assert.NoError(t, err)
	assert.Equal(t, "default-cluster-name", sd.GetClusterName())
}

func TestServiceDiscovery_ListServices(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset(
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc1",
				Namespace: "test-ns",
			},
		},
		&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc2",
				Namespace: "test-ns",
			},
		},
	)

	sd := NewServiceDiscovery(clientset, WithNamespace("test-ns"))
	services, err := sd.ListServices(ctx)

	assert.NoError(t, err)
	assert.Len(t, services.Items, 2)
}

func TestServiceDiscovery_GenerateDomainMapping(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	sd := NewServiceDiscovery(clientset, WithClusterName("test-cluster"))

	serviceList := &v1.ServiceList{
		Items: []v1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "test-ns",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service2",
					Namespace: "test-ns",
				},
			},
		},
	}

	result := sd.GenerateDomainMapping(serviceList)

	// Now generates 2 domain types per service: clusterset and cluster-specific
	assert.Len(t, result.RemoteDomains, 4)
	assert.Len(t, result.DomainMapping, 4)

	// Check clusterset domains
	assert.Contains(t, result.RemoteDomains, "service1.test-ns.svc.clusterset.remote")
	assert.Equal(t, "service1.test-ns.svc.cluster.local", result.DomainMapping["service1.test-ns.svc.clusterset.remote"])
	assert.Contains(t, result.RemoteDomains, "service2.test-ns.svc.clusterset.remote")
	assert.Equal(t, "service2.test-ns.svc.cluster.local", result.DomainMapping["service2.test-ns.svc.clusterset.remote"])

	// Check cluster-specific domains
	assert.Contains(t, result.RemoteDomains, "service1.test-ns.svc.test-cluster.remote")
	assert.Equal(t, "service1.test-ns.svc.cluster.local", result.DomainMapping["service1.test-ns.svc.test-cluster.remote"])
	assert.Contains(t, result.RemoteDomains, "service2.test-ns.svc.test-cluster.remote")
	assert.Equal(t, "service2.test-ns.svc.cluster.local", result.DomainMapping["service2.test-ns.svc.test-cluster.remote"])
}

func TestServiceDiscovery_GenerateDomainMapping_Nil(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	sd := NewServiceDiscovery(clientset)

	result := sd.GenerateDomainMapping(nil)

	assert.Empty(t, result.RemoteDomains)
	assert.Empty(t, result.DomainMapping)
}

func TestServiceDiscovery_GenerateDomainMapping_Empty(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	sd := NewServiceDiscovery(clientset)

	serviceList := &v1.ServiceList{Items: []v1.Service{}}
	result := sd.GenerateDomainMapping(serviceList)

	assert.Empty(t, result.RemoteDomains)
	assert.Empty(t, result.DomainMapping)
}

func TestServiceDiscovery_GenerateServiceDomainMapping(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	sd := NewServiceDiscovery(clientset, WithClusterName("my-cluster"))

	service := v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-service",
			Namespace: "my-ns",
		},
	}

	// Test clusterset domain mapping
	clustersetMapping := sd.generateClustersetDomainMapping(service)
	assert.Equal(t, "my-service.my-ns.svc.clusterset.remote", clustersetMapping.RemoteDomain)
	assert.Equal(t, "my-service.my-ns.svc.cluster.local", clustersetMapping.LocalDomain)

	// Test cluster domain mapping
	clusterMapping := sd.generateClusterDomainMapping(service)
	assert.Equal(t, "my-service.my-ns.svc.my-cluster.remote", clusterMapping.RemoteDomain)
	assert.Equal(t, "my-service.my-ns.svc.cluster.local", clusterMapping.LocalDomain)
}

func TestDomainMappingResult(t *testing.T) {
	result := DomainMappingResult{
		RemoteDomains: []string{"remote1", "remote2"},
		DomainMapping: map[string]string{
			"remote1": "local1",
			"remote2": "local2",
		},
	}

	assert.Len(t, result.RemoteDomains, 2)
	assert.Len(t, result.DomainMapping, 2)
	assert.Equal(t, "local1", result.DomainMapping["remote1"])
}
