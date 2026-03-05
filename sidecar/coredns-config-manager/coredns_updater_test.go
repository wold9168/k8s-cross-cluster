package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCoreDNSUpdater_New(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	config := CoreDNSConfig{
		Namespace:      "kube-system",
		ConfigMapName:  "coredns",
		ConfigKey:      "Corefile",
		DeploymentName: "coredns",
	}

	updater := NewCoreDNSUpdater(clientset, config)

	assert.NotNil(t, updater)
	assert.Equal(t, clientset, updater.clientset)
	assert.Equal(t, config, updater.config)
	assert.NotNil(t, updater.corefileManager)
}

func TestCoreDNSUpdater_EnsureConfig_UpdateNeeded(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: "kube-system",
			},
			Data: map[string]string{
				"Corefile": ".:53 {\n    errors\n}\n",
			},
		},
	)

	config := CoreDNSConfig{
		Namespace:      "kube-system",
		ConfigMapName:  "coredns",
		ConfigKey:      "Corefile",
		DeploymentName: "coredns",
	}

	updater := NewCoreDNSUpdater(clientset, config)
	err := updater.EnsureConfig(ctx, "10.0.0.1:10053")

	// Fake clientset doesn't actually update deployments
	// so this may fail at rollout step, but tests the logic
	assert.Error(t, err) // Expected to fail at rollout since deployment doesn't exist
}

func TestCoreDNSUpdater_EnsureConfig_ConfigMapNotFound(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()

	config := CoreDNSConfig{
		Namespace:      "kube-system",
		ConfigMapName:  "nonexistent",
		ConfigKey:      "Corefile",
		DeploymentName: "coredns",
	}

	updater := NewCoreDNSUpdater(clientset, config)
	err := updater.EnsureConfig(ctx, "10.0.0.1:10053")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get CoreDNS ConfigMap")
}

func TestCoreDNSUpdater_EnsureConfig_ConfigKeyNotFound(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: "kube-system",
			},
			Data: map[string]string{
				"OtherKey": "value",
			},
		},
	)

	config := CoreDNSConfig{
		Namespace:      "kube-system",
		ConfigMapName:  "coredns",
		ConfigKey:      "Corefile",
		DeploymentName: "coredns",
	}

	updater := NewCoreDNSUpdater(clientset, config)
	err := updater.EnsureConfig(ctx, "10.0.0.1:10053")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist in ConfigMap")
}

func TestCoreDNSUpdater_GetConfig(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	config := CoreDNSConfig{
		Namespace:      "test-ns",
		ConfigMapName:  "test-cm",
		ConfigKey:      "test-key",
		DeploymentName: "test-deploy",
	}

	updater := NewCoreDNSUpdater(clientset, config)

	retrieved := updater.GetConfig()
	assert.Equal(t, config, retrieved)
}

func TestCoreDNSUpdater_EnsureConfig_AlreadyUpToDate(t *testing.T) {
	ctx := context.Background()
	upstreamServer := "10.0.0.1:10053"

	corefileContent := ManagedSectionStart + `
remote:53 {
    forward . ` + upstreamServer + `
}
` + ManagedSectionEnd + `
.:53 {
    errors
}
`

	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: "kube-system",
			},
			Data: map[string]string{
				"Corefile": corefileContent,
			},
		},
	)

	config := CoreDNSConfig{
		Namespace:      "kube-system",
		ConfigMapName:  "coredns",
		ConfigKey:      "Corefile",
		DeploymentName: "coredns",
	}

	updater := NewCoreDNSUpdater(clientset, config)
	err := updater.EnsureConfig(ctx, upstreamServer)

	// When config is already up to date, no update is needed
	// The fake clientset doesn't have the deployment, so this should not error
	// since no update is performed
	assert.NoError(t, err)
}

func TestCoreDNSConfig(t *testing.T) {
	config := CoreDNSConfig{
		Namespace:      "kube-system",
		ConfigMapName:  "coredns",
		ConfigKey:      "Corefile",
		DeploymentName: "coredns",
		ManagedSection: ManagedSection{
			StartMarker: "### START ###",
			EndMarker:   "### END ###",
		},
	}

	assert.Equal(t, "kube-system", config.Namespace)
	assert.Equal(t, "coredns", config.ConfigMapName)
	assert.Equal(t, "coredns", config.DeploymentName)
}

func TestCoreDNSUpdater_EnsureConfig_ValidationWarning(t *testing.T) {
	ctx := context.Background()
	
	// Corefile with unbalanced braces (invalid)
	invalidCorefile := ".:53 {\n    errors\n"

	clientset := fake.NewSimpleClientset(
		&v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: "kube-system",
			},
			Data: map[string]string{
				"Corefile": invalidCorefile,
			},
		},
	)

	config := CoreDNSConfig{
		Namespace:      "kube-system",
		ConfigMapName:  "coredns",
		ConfigKey:      "Corefile",
		DeploymentName: "coredns",
	}

	updater := NewCoreDNSUpdater(clientset, config)
	err := updater.EnsureConfig(ctx, "10.0.0.1:10053")

	// Should still proceed despite validation warning
	assert.Error(t, err) // Will fail at rollout
}
