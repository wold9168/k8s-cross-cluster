package generator

import (
	"strings"

	"k8s.io/klog/v2"
)

// GenerateCaddyConfig 从远程域名及其映射生成 Caddy 配置
// 配置格式:
// <remote-domain> {
//     tls internal
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
		builder.WriteString("    tls internal\n")
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
