package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorefileManager_New(t *testing.T) {
	config := CoreDNSConfig{
		Namespace:     "kube-system",
		ConfigMapName: "coredns",
		ConfigKey:     "Corefile",
	}

	manager := NewCorefileManager(config)

	assert.NotNil(t, manager)
	assert.Equal(t, config, manager.config)
}

func TestCorefileManager_ParseCorefile(t *testing.T) {
	config := CoreDNSConfig{}
	manager := NewCorefileManager(config)

	content := `.:53 {
    errors
    health
}

example.com:53 {
    log
}
`

	blocks := manager.ParseCorefile(content)

	assert.GreaterOrEqual(t, len(blocks), 2)
}

func TestCorefileManager_NeedsUpdate(t *testing.T) {
	config := CoreDNSConfig{}
	manager := NewCorefileManager(config)

	upstreamServer := "10.0.0.1:10053"

	// Test without managed section
	content := ".:53 { errors }"
	assert.True(t, manager.NeedsUpdate(content, upstreamServer))

	// Test with managed section but wrong upstream
	contentWithSection := ManagedSectionStart + `
remote:53 {
    forward . 10.0.0.2:10053
}
` + ManagedSectionEnd
	assert.True(t, manager.NeedsUpdate(contentWithSection, upstreamServer))

	// Test with correct upstream
	contentCorrect := ManagedSectionStart + `
remote:53 {
    forward . 10.0.0.1:10053
}
` + ManagedSectionEnd
	assert.False(t, manager.NeedsUpdate(contentCorrect, upstreamServer))
}

func TestCorefileManager_UpdateCorefile(t *testing.T) {
	config := CoreDNSConfig{}
	manager := NewCorefileManager(config)

	upstreamServer := "10.0.0.1:10053"
	originalContent := ".:53 {\n    errors\n}\n"

	updated := manager.UpdateCorefile(originalContent, upstreamServer)

	assert.Contains(t, updated, ManagedSectionStart)
	assert.Contains(t, updated, ManagedSectionEnd)
	assert.Contains(t, updated, upstreamServer)
	assert.Contains(t, updated, "remote:53")
	assert.Contains(t, updated, "forward .")
}

func TestCorefileManager_UpdateCorefile_ReplaceExisting(t *testing.T) {
	config := CoreDNSConfig{}
	manager := NewCorefileManager(config)

	oldUpstream := "10.0.0.1:10053"
	newUpstream := "10.0.0.2:10053"

	contentWithOld := ManagedSectionStart + `
remote:53 {
    forward . ` + oldUpstream + `
}
` + ManagedSectionEnd + `
.:53 {
    errors
}
`

	updated := manager.UpdateCorefile(contentWithOld, newUpstream)

	assert.Contains(t, updated, newUpstream)
	assert.NotContains(t, updated, oldUpstream)
}

func TestCorefileManager_ValidateCorefile(t *testing.T) {
	config := CoreDNSConfig{}
	manager := NewCorefileManager(config)

	// Valid corefile
	validContent := ".:53 {\n    errors\n}\n"
	err := manager.ValidateCorefile(validContent)
	assert.NoError(t, err)

	// Invalid corefile - unbalanced braces
	invalidContent := ".:53 {\n    errors\n"
	err = manager.ValidateCorefile(invalidContent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unbalanced braces")
}

func TestParseCorefile(t *testing.T) {
	content := `.:53 {
    errors
    health
}

example.com:53 {
    log
}
`

	blocks := parseCorefile(content)

	assert.GreaterOrEqual(t, len(blocks), 1)
}

func TestParseCorefile_Empty(t *testing.T) {
	blocks := parseCorefile("")
	assert.Empty(t, blocks)
}

func TestCreateRemoteDomainBlock(t *testing.T) {
	upstreamServer := "10.0.0.1:10053"

	block := createRemoteDomainBlock(upstreamServer)

	assert.Contains(t, block, ManagedSectionStart)
	assert.Contains(t, block, ManagedSectionEnd)
	assert.Contains(t, block, "remote:53")
	assert.Contains(t, block, upstreamServer)
	assert.Contains(t, block, "forward .")
	assert.Contains(t, block, "cache 30")
}

func TestRemoveManagedSection(t *testing.T) {
	content := ManagedSectionStart + `
remote:53 {
    forward . 10.0.0.1:10053
}
` + ManagedSectionEnd + `
.:53 {
    errors
}
`

	result := removeManagedSection(content)

	assert.NotContains(t, result, ManagedSectionStart)
	assert.NotContains(t, result, ManagedSectionEnd)
	assert.Contains(t, result, ".:53")
	assert.Contains(t, result, "errors")
}

func TestRemoveManagedSection_NotPresent(t *testing.T) {
	content := ".:53 {\n    errors\n}\n"

	result := removeManagedSection(content)

	assert.Equal(t, content, result)
}

func TestIsManagedSectionPresent(t *testing.T) {
	content := ManagedSectionStart + "\n...\n" + ManagedSectionEnd

	assert.True(t, isManagedSectionPresent(content))
	assert.False(t, isManagedSectionPresent(".:53 { errors }"))
}

func TestIsUpstreamCorrect(t *testing.T) {
	upstream := "10.0.0.1:10053"

	content := ManagedSectionStart + `
remote:53 {
    forward . ` + upstream + `
}
` + ManagedSectionEnd

	assert.True(t, isUpstreamCorrect(content, upstream))
	assert.False(t, isUpstreamCorrect(content, "10.0.0.2:10053"))
}

func TestIsUpstreamCorrect_NoManagedSection(t *testing.T) {
	content := ".:53 { errors }"
	assert.False(t, isUpstreamCorrect(content, "10.0.0.1:10053"))
}

func TestNeedsUpdate(t *testing.T) {
	upstream := "10.0.0.1:10053"

	// No managed section
	content1 := ".:53 { errors }"
	assert.True(t, needsUpdate(content1, upstream))

	// Wrong upstream
	content2 := ManagedSectionStart + `
remote:53 {
    forward . 10.0.0.2:10053
}
` + ManagedSectionEnd
	assert.True(t, needsUpdate(content2, upstream))

	// Correct upstream
	content3 := ManagedSectionStart + `
remote:53 {
    forward . ` + upstream + `
}
` + ManagedSectionEnd
	assert.False(t, needsUpdate(content3, upstream))
}

func TestUpdateCorefile(t *testing.T) {
	upstream := "10.0.0.1:10053"
	content := ".:53 { errors }"

	updated := updateCorefile(content, upstream)

	assert.Contains(t, updated, ManagedSectionStart)
	assert.Contains(t, updated, upstream)
}

func TestCorefileError(t *testing.T) {
	err := &CorefileError{
		Message: "test error",
		Details: map[string]int{"open": 2, "close": 1},
	}

	assert.Equal(t, "test error", err.Error())
	assert.Equal(t, 2, err.Details["open"])
}

func TestCorefileManager_ParseCorefile_Complex(t *testing.T) {
	config := CoreDNSConfig{}
	manager := NewCorefileManager(config)

	content := `.:53 {
    errors
    health {
        lameduck 5s
    }
    kubernetes cluster.local in-addr.arpa ip6.arpa {
        pods insecure
        fallthrough in-addr.arpa ip6.arpa
    }
    prometheus :9153
    forward . /etc/resolv.conf
    cache 30
    loop
    reload
    loadbalance
}
`

	blocks := manager.ParseCorefile(content)
	assert.GreaterOrEqual(t, len(blocks), 1)
	
	// Verify all blocks have balanced braces
	for _, block := range blocks {
		openCount := strings.Count(block, "{")
		closeCount := strings.Count(block, "}")
		assert.Equal(t, openCount, closeCount, "Block should have balanced braces")
	}
}
