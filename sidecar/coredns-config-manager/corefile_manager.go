package main

import (
	"strings"

	"k8s.io/klog/v2"
)

// CorefileManager manages CoreDNS Corefile configuration
type CorefileManager struct {
	config CoreDNSConfig
}

// NewCorefileManager creates a new CorefileManager
func NewCorefileManager(config CoreDNSConfig) *CorefileManager {
	return &CorefileManager{
		config: config,
	}
}

// ParseCorefile parses Corefile content into server blocks
func (cm *CorefileManager) ParseCorefile(content string) []string {
	return parseCorefile(content)
}

// NeedsUpdate checks if Corefile needs to be updated with new upstream
func (cm *CorefileManager) NeedsUpdate(content, upstreamServer string) bool {
	return needsUpdate(content, upstreamServer)
}

// UpdateCorefile updates Corefile with remote domain forwarding configuration
func (cm *CorefileManager) UpdateCorefile(content, upstreamServer string) string {
	return updateCorefile(content, upstreamServer)
}

// ValidateCorefile validates Corefile structure
func (cm *CorefileManager) ValidateCorefile(content string) error {
	// Basic validation: check for balanced braces
	openBraces := strings.Count(content, "{")
	closeBraces := strings.Count(content, "}")
	if openBraces != closeBraces {
		return &CorefileError{
			Message: "unbalanced braces in Corefile",
			Details: map[string]int{"open": openBraces, "close": closeBraces},
		}
	}
	return nil
}

// CorefileError represents a Corefile parsing or validation error
type CorefileError struct {
	Message string
	Details map[string]int
}

func (e *CorefileError) Error() string {
	return e.Message
}

// parseCorefile parses Corefile content and returns slice of server blocks
func parseCorefile(content string) []string {
	blocks := []string{}

	// Simple block extraction - can be enhanced with proper parsing
	lines := strings.Split(content, "\n")
	var currentBlock strings.Builder
	bracketCount := 0

	for _, line := range lines {
		currentBlock.WriteString(line)
		currentBlock.WriteString("\n")

		bracketCount += strings.Count(line, "{")
		bracketCount -= strings.Count(line, "}")

		if bracketCount == 0 && currentBlock.Len() > 0 {
			block := strings.TrimSpace(currentBlock.String())
			if block != "" {
				blocks = append(blocks, block)
			}
			currentBlock.Reset()
		}
	}

	return blocks
}

// needsUpdate determines if Corefile requires update
func needsUpdate(content, upstreamServer string) bool {
	isManagedSectionPresentStatus := isManagedSectionPresent(content)
	isUpstreamCorrectStatus := isUpstreamCorrect(content, upstreamServer)
	klog.V(4).Infof("Corefile needs update: managed section present: %t, upstream correct: %t",
		isManagedSectionPresentStatus, isUpstreamCorrectStatus)
	return !isManagedSectionPresentStatus || !isUpstreamCorrectStatus
}

// updateCorefile adds or updates the *.remote domain configuration
func updateCorefile(content, upstreamServer string) string {
	// First, remove any existing managed section
	contentWithoutManaged := removeManagedSection(content)

	// Generate new managed section
	newBlock := createRemoteDomainBlock(upstreamServer)

	return newBlock + contentWithoutManaged
}

// isManagedSectionPresent checks if managed section exists in content
func isManagedSectionPresent(content string) bool {
	return strings.Contains(content, ManagedSectionStart) && strings.Contains(content, ManagedSectionEnd)
}

// isUpstreamCorrect checks if upstream server matches expected value
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

// removeManagedSection removes existing managed section from content
func removeManagedSection(content string) string {
	startIdx := strings.Index(content, ManagedSectionStart)
	if startIdx == -1 {
		return content
	}

	endIdx := strings.Index(content, ManagedSectionEnd)
	if endIdx == -1 {
		return content // Malformed managed section
	}

	// Include newline after end marker for proper formatting
	endIdx += len(ManagedSectionEnd)
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	return content[:startIdx] + content[endIdx:]
}

// createRemoteDomainBlock creates a server block for *.remote domain
func createRemoteDomainBlock(upstreamServer string) string {
	block := ManagedSectionStart + "\n"
	block += "remote:53 {\n"
	block += "    log\n"
	block += "    errors\n"
	block += "    forward . " + upstreamServer + "\n"
	block += "    cache 30\n"
	block += "}\n"
	block += ManagedSectionEnd + "\n"
	return block
}
