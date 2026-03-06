package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCaddyConfigGenerator_New(t *testing.T) {
	gen := NewCaddyConfigGenerator()

	assert.NotNil(t, gen)
	assert.NotNil(t, gen.Template)
	assert.IsType(t, DefaultCaddyTemplate{}, gen.Template)
}

func TestCaddyConfigGenerator_WithCustomTemplate(t *testing.T) {
	customTemplate := &CustomTemplate{}
	gen := NewCaddyConfigGeneratorWithTemplate(customTemplate)

	assert.NotNil(t, gen)
	assert.Equal(t, customTemplate, gen.Template)
}

func TestCaddyConfigGenerator_Generate(t *testing.T) {
	gen := NewCaddyConfigGenerator()

	remoteDomains := []string{
		"svc1.ns.svc.cluster1.remote",
		"svc2.ns.svc.cluster1.remote",
	}
	domainMapping := map[string]string{
		"svc1.ns.svc.cluster1.remote": "svc1.ns.svc.cluster.local",
		"svc2.ns.svc.cluster1.remote": "svc2.ns.svc.cluster.local",
	}

	config := gen.Generate(remoteDomains, domainMapping)

	assert.Contains(t, config, "svc1.ns.svc.cluster1.remote")
	assert.Contains(t, config, "reverse_proxy svc1.ns.svc.cluster.local")
	assert.Contains(t, config, "svc2.ns.svc.cluster1.remote")
	assert.Contains(t, config, "reverse_proxy svc2.ns.svc.cluster.local")
	assert.Contains(t, config, "tls internal")
}

func TestCaddyConfigGenerator_Generate_Empty(t *testing.T) {
	gen := NewCaddyConfigGenerator()

	config := gen.Generate([]string{}, map[string]string{})

	assert.Equal(t, "", config)
}

func TestCaddyConfigGenerator_Generate_MissingMapping(t *testing.T) {
	gen := NewCaddyConfigGenerator()

	remoteDomains := []string{"svc1.ns.svc.cluster1.remote"}
	domainMapping := map[string]string{} // Missing mapping

	config := gen.Generate(remoteDomains, domainMapping)

	assert.Equal(t, "", config) // Should skip due to missing mapping
}

func TestCaddyConfigGenerator_GenerateFromResult(t *testing.T) {
	gen := NewCaddyConfigGenerator()

	result := DomainMappingResult{
		RemoteDomains: []string{"svc1.ns.svc.cluster1.remote"},
		DomainMapping: map[string]string{
			"svc1.ns.svc.cluster1.remote": "svc1.ns.svc.cluster.local",
		},
	}

	config := gen.GenerateFromResult(result)

	assert.Contains(t, config, "svc1.ns.svc.cluster1.remote")
	assert.Contains(t, config, "reverse_proxy svc1.ns.svc.cluster.local")
}

func TestDefaultCaddyTemplate_Generate(t *testing.T) {
	template := DefaultCaddyTemplate{}

	config := template.Generate("remote.example.com", "local.example.com")

	assert.Contains(t, config, "remote.example.com")
	assert.Contains(t, config, "reverse_proxy local.example.com")
	assert.Contains(t, config, "tls internal")
	assert.Contains(t, config, "{")
	assert.Contains(t, config, "}")
}

func TestDefaultCaddyTemplate_Generate_Format(t *testing.T) {
	template := DefaultCaddyTemplate{}

	config := template.Generate("remote.test.com", "local.test.com")

	lines := strings.Split(config, "\n")
	assert.GreaterOrEqual(t, len(lines), 5) // At least 5 lines
	assert.Contains(t, lines[0], "remote.test.com")
	assert.Contains(t, lines[1], "tls internal")
	assert.Contains(t, lines[2], "reverse_proxy")
}

// CustomTemplate is a mock template for testing
type CustomTemplate struct{}

func (t *CustomTemplate) Generate(remoteDomain, localDomain string) string {
	return "custom: " + remoteDomain + " -> " + localDomain
}

func TestCaddyConfigGenerator_CustomTemplate(t *testing.T) {
	customTemplate := &CustomTemplate{}
	gen := NewCaddyConfigGeneratorWithTemplate(customTemplate)

	config := gen.Generate([]string{"remote.com"}, map[string]string{"remote.com": "local.com"})

	assert.Contains(t, config, "custom: remote.com -> local.com")
}

func TestCaddyConfig_GenerateCaddyConfig(t *testing.T) {
	config := CaddyConfig{
		RemoteDomain: "remote.example.com",
		LocalDomain:  "local.example.com",
		ConfigBlock:  "block content",
	}

	assert.Equal(t, "remote.example.com", config.RemoteDomain)
	assert.Equal(t, "local.example.com", config.LocalDomain)
	assert.Equal(t, "block content", config.ConfigBlock)
}
