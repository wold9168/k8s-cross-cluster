package main

import (
	"strings"

	"k8s.io/klog/v2"
)

// GenerateCaddyConfig generates Caddy configuration from remote domains and their mappings
// The configuration format:
// <remote-domain> {
//     reverse_proxy <local-domain>
// }
func GenerateCaddyConfig(remoteDomains []string, domainMapping map[string]string) string {
	var builder strings.Builder

	for _, remoteDomain := range remoteDomains {
		localDomain, exists := domainMapping[remoteDomain]
		if !exists {
			klog.Warningf("No mapping found for remote domain: %s, skipping", remoteDomain)
			continue
		}

		builder.WriteString(remoteDomain)
		builder.WriteString(" {\n")
		builder.WriteString("    reverse_proxy ")
		builder.WriteString(localDomain)
		builder.WriteString("\n}\n")
	}

	config := builder.String()
	if config != "" {
		klog.Infof("Generated Caddy configuration with %d domain(s)", len(remoteDomains))
	} else {
		klog.Warning("Generated empty Caddy configuration")
	}

	return config
}