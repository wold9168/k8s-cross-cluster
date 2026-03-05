package generator

import (
	"strings"

	"k8s.io/klog/v2"
)

// CaddyConfigGenerator generates Caddy proxy configurations
type CaddyConfigGenerator struct {
	// Template allows customizing the generated config format
	Template CaddyTemplate
}

// CaddyTemplate defines the template for generating Caddy config blocks
type CaddyTemplate interface {
	Generate(remoteDomain, localDomain string) string
}

// DefaultCaddyTemplate is the default template implementation
type DefaultCaddyTemplate struct{}

// Generate generates a Caddy config block with default format
func (t DefaultCaddyTemplate) Generate(remoteDomain, localDomain string) string {
	var builder strings.Builder
	builder.WriteString(remoteDomain)
	builder.WriteString(" {\n")
	builder.WriteString("    tls internal\n")
	builder.WriteString("    reverse_proxy ")
	builder.WriteString(localDomain)
	builder.WriteString("\n}\n")
	return builder.String()
}

// NewCaddyConfigGenerator creates a new CaddyConfigGenerator
func NewCaddyConfigGenerator() *CaddyConfigGenerator {
	return &CaddyConfigGenerator{
		Template: DefaultCaddyTemplate{},
	}
}

// NewCaddyConfigGeneratorWithTemplate creates a generator with custom template
func NewCaddyConfigGeneratorWithTemplate(template CaddyTemplate) *CaddyConfigGenerator {
	return &CaddyConfigGenerator{
		Template: template,
	}
}

// Generate generates Caddy configuration from domain mappings
func (g *CaddyConfigGenerator) Generate(remoteDomains []string, domainMapping map[string]string) string {
	var builder strings.Builder

	for _, remoteDomain := range remoteDomains {
		localDomain, exists := domainMapping[remoteDomain]
		if !exists {
			klog.Warningf("No mapping found for remote domain: %s, skipping", remoteDomain)
			continue
		}

		configBlock := g.Template.Generate(remoteDomain, localDomain)
		builder.WriteString(configBlock)
	}

	config := builder.String()
	if config != "" {
		klog.Infof("Generated Caddy configuration with %d domain(s)", len(remoteDomains))
	} else {
		klog.Warning("Generated empty Caddy configuration")
	}

	return config
}

// GenerateFromResult generates Caddy configuration from DomainMappingResult
func (g *CaddyConfigGenerator) GenerateFromResult(result DomainMappingResult) string {
	return g.Generate(result.RemoteDomains, result.DomainMapping)
}
