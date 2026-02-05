package main

import (
	"regexp"
	"strings"

	"k8s.io/klog/v2"
)

// parseCorefile parses the Corefile content and returns a slice of server blocks
func parseCorefile(content string) []string {
	// Split the content by } to separate server blocks
	// This is a simplified parsing approach - in a production environment,
	// you might want to use a more sophisticated parser
	blocks := []string{}

	// Find all server blocks (blocks that start with a domain and end with })
	re := regexp.MustCompile(`(?s)([^{}]*\{.*?\})`)
	matches := re.FindAllString(content, -1)

	for _, match := range matches {
		blocks = append(blocks, strings.TrimSpace(match))
	}

	return blocks
}

// findServerBlock finds a server block that matches the given domain pattern
func findServerBlock(blocks []string, domainPattern string) (string, int) {
	for i, block := range blocks {
		// Check if the block starts with the domain pattern
		lines := strings.Split(block, "\n")
		if len(lines) > 0 {
			firstLine := strings.TrimSpace(lines[0])
			// The first line should contain the domain
			if strings.Contains(firstLine, domainPattern) {
				return block, i
			}
		}
	}
	return "", -1
}

// hasUpstreamConfig checks if a server block already has an upstream configuration
func hasUpstreamConfig(block, upstreamServer string) bool {
	return strings.Contains(block, "forward") && strings.Contains(block, upstreamServer)
}

// createRemoteDomainBlock creates a server block for *.remote domains that forwards to the specified upstream
func createRemoteDomainBlock(upstreamServer string) string {
	block := ManagedSectionStart + "\n"
	block += "remote {\n"
	block += "    log\n"
	block += "    errors\n"
	block += "    forward . " + upstreamServer + "\n"
	block += "    cache 30\n"
	block += "}\n"
	block += ManagedSectionEnd + "\n"
	return block
}

// removeManagedSection removes any existing managed section from the Corefile content
func removeManagedSection(content string) string {
	startIdx := strings.Index(content, ManagedSectionStart)
	if startIdx == -1 {
		return content // No managed section found
	}

	endIdx := strings.Index(content, ManagedSectionEnd)
	if endIdx == -1 {
		return content // Malformed managed section, return as is
	}

	// Include the newline after the end marker for proper formatting
	endIdx += len(ManagedSectionEnd)
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	// Return content before the start and after the end
	return content[:startIdx] + content[endIdx:]
}

// updateCorefile adds or updates the configuration for *.remote domains to forward to the specified upstream server
func updateCorefile(content, upstreamServer string) string {
	// First, remove any existing managed section
	contentWithoutManaged := removeManagedSection(content)

	// Add the new managed section at the end
	newBlock := createRemoteDomainBlock(upstreamServer)

	// Ensure there's a newline between the existing content and the new block
	if !strings.HasSuffix(strings.TrimSpace(contentWithoutManaged), "{") &&
		!strings.HasPrefix(newBlock, "\n") {
		newBlock = "\n" + newBlock
	}

	return newBlock + contentWithoutManaged
}

// isManagedSectionPresent checks if the managed section already exists in the content
func isManagedSectionPresent(content string) bool {
	return strings.Contains(content, ManagedSectionStart) && strings.Contains(content, ManagedSectionEnd)
}

// isUpstreamCorrect checks if the upstream server in the managed section matches the expected server
func isUpstreamCorrect(content, expectedUpstream string) bool {
	if !isManagedSectionPresent(content) {
		return false
	}

	startIdx := strings.Index(content, ManagedSectionStart)
	endIdx := strings.Index(content, ManagedSectionEnd) + len(ManagedSectionEnd)

	if startIdx == -1 || endIdx == -1 {
		return false
	}

	managedSection := content[startIdx:endIdx]
	return strings.Contains(managedSection, "forward . "+expectedUpstream)
}

// checkCoreDNSConfig checks if the CoreDNS configuration already contains our upstream configuration
func checkCoreDNSConfig(content, upstreamServer string) bool {
	// Parse the Corefile content
	blocks := parseCorefile(content)

	// Look for a server block that handles *.remote domains
	remoteBlock, idx := findServerBlock(blocks, "*.remote")
	if idx == -1 {
		// No server block for *.remote found, so configuration is missing
		return false
	}

	// Check if this block has the correct upstream configuration
	return hasUpstreamConfig(remoteBlock, upstreamServer)
}

// needsUpdate determines if the Corefile needs to be updated
func needsUpdate(content, upstreamServer string) bool {
	isManagedSectionPresentStatus := isManagedSectionPresent(content)
	isUpstreamCorrectStatus := isUpstreamCorrect(content, upstreamServer)
	klog.V(4).Infof("Corefile needs update: managed section present: %t, upstream correct: %t",
		isManagedSectionPresentStatus, isUpstreamCorrectStatus)
	return !isManagedSectionPresentStatus || !isUpstreamCorrectStatus
}
