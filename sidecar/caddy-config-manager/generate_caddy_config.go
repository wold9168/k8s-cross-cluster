package main

// GenerateCaddyConfig generates Caddy configuration from domain mappings
// This is a legacy function kept for backward compatibility
// Deprecated: Use CaddyConfigGenerator.Generate instead
func GenerateCaddyConfig(remoteDomains []string, domainMapping map[string]string) string {
	generator := NewCaddyConfigGenerator()
	return generator.Generate(remoteDomains, domainMapping)
}
