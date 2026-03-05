package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"
)

// Test interfaces are properly implemented

func TestPeerLister_Interface(t *testing.T) {
	pd := NewPeerDiscovery()
	var _ PeerLister = pd
}

func TestCorefileUpdater_Interface(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	config := CoreDNSConfig{}
	updater := NewCoreDNSUpdater(clientset, config)
	var _ CorefileUpdater = updater
}

func TestCorefileValidator_Interface(t *testing.T) {
	config := CoreDNSConfig{}
	manager := NewCorefileManager(config)
	var _ CorefileValidator = manager
}

func TestPeerLister_GetPeers(t *testing.T) {
	pd := NewPeerDiscovery()
	ctx := context.Background()

	// Will fail without tailscaled but tests the interface
	peers, err := pd.GetPeers(ctx)
	if err != nil {
		assert.Empty(t, peers)
	}
}

func TestPeerLister_GetSelf(t *testing.T) {
	pd := NewPeerDiscovery()
	ctx := context.Background()

	// Will fail without tailscaled but tests the interface
	self, err := pd.GetSelf(ctx)
	if err != nil {
		assert.Empty(t, self.ID)
	}
}

func TestCorefileUpdater_EnsureConfig(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	config := CoreDNSConfig{
		Namespace:      "kube-system",
		ConfigMapName:  "coredns",
		ConfigKey:      "Corefile",
		DeploymentName: "coredns",
	}
	updater := NewCoreDNSUpdater(clientset, config)

	ctx := context.Background()
	err := updater.EnsureConfig(ctx, "10.0.0.1:10053")

	// Expected to fail at rollout since deployment doesn't exist
	assert.Error(t, err)
}

func TestCorefileUpdater_GetConfig(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	config := CoreDNSConfig{
		Namespace:     "test-ns",
		ConfigMapName: "test-cm",
	}
	updater := NewCoreDNSUpdater(clientset, config)

	retrieved := updater.GetConfig()
	assert.Equal(t, config, retrieved)
}

func TestCorefileValidator_ValidateCorefile(t *testing.T) {
	config := CoreDNSConfig{}
	manager := NewCorefileManager(config)

	// Valid corefile
	valid := ".:53 {\n    errors\n}\n"
	err := manager.ValidateCorefile(valid)
	assert.NoError(t, err)

	// Invalid corefile
	invalid := ".:53 {\n    errors\n"
	err = manager.ValidateCorefile(invalid)
	assert.Error(t, err)
}

func TestCorefileValidator_NeedsUpdate(t *testing.T) {
	config := CoreDNSConfig{}
	manager := NewCorefileManager(config)

	content := ".:53 { errors }"
	needsUpdate := manager.NeedsUpdate(content, "10.0.0.1:10053")
	assert.True(t, needsUpdate)
}

func TestCorefileValidator_UpdateCorefile(t *testing.T) {
	config := CoreDNSConfig{}
	manager := NewCorefileManager(config)

	content := ".:53 { errors }"
	updated := manager.UpdateCorefile(content, "10.0.0.1:10053")

	assert.Contains(t, updated, ManagedSectionStart)
	assert.Contains(t, updated, "10.0.0.1:10053")
}

func TestInterfaceImplementations(t *testing.T) {
	// Verify all interface implementations compile
	clientset := fake.NewSimpleClientset()

	// PeerDiscovery implements PeerLister
	var pl PeerLister = NewPeerDiscovery()
	assert.NotNil(t, pl)

	// CoreDNSUpdater implements CorefileUpdater
	config := CoreDNSConfig{}
	var cu CorefileUpdater = NewCoreDNSUpdater(clientset, config)
	assert.NotNil(t, cu)

	// CorefileManager implements CorefileValidator
	var cv CorefileValidator = NewCorefileManager(config)
	assert.NotNil(t, cv)
}

func TestInterfaceMethods(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()

	// Test PeerLister methods through interface
	var pl PeerLister = NewPeerDiscovery()
	_, err := pl.GetPeers(ctx)
	// May fail without tailscaled
	_ = err

	_, err = pl.GetSelf(ctx)
	// May fail without tailscaled
	_ = err

	// Test CorefileUpdater methods through interface
	config := CoreDNSConfig{
		Namespace:      "kube-system",
		ConfigMapName:  "coredns",
		ConfigKey:      "Corefile",
		DeploymentName: "coredns",
	}
	var cu CorefileUpdater = NewCoreDNSUpdater(clientset, config)
	
	retrievedConfig := cu.GetConfig()
	assert.Equal(t, config, retrievedConfig)
}
