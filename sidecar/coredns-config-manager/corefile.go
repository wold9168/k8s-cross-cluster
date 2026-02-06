package main

import (
	"regexp"
	"strings"

	"k8s.io/klog/v2"
)

// parseCorefile 解析 Corefile 内容并返回服务器块切片
func parseCorefile(content string) []string {
	// 按花括号（}）分割内容以分离服务器块
	blocks := []string{}

	// 查找所有服务器块（以域名开头并以 } 结尾的块）
	re := regexp.MustCompile(`(?s)([^{}]*\{.*?\})`)
	matches := re.FindAllString(content, -1)

	for _, match := range matches {
		blocks = append(blocks, strings.TrimSpace(match))
	}

	return blocks
}

// findServerBlock 查找与给定域名模式匹配的服务器块
func findServerBlock(blocks []string, domainPattern string) (string, int) {
	for i, block := range blocks {
		// 检查块是否以域名模式开头
		lines := strings.Split(block, "\n")
		if len(lines) > 0 {
			firstLine := strings.TrimSpace(lines[0])
			// 第一行应包含域名
			if strings.Contains(firstLine, domainPattern) {
				return block, i
			}
		}
	}
	return "", -1
}

// hasUpstreamConfig 检查服务器块是否已具有上游配置
func hasUpstreamConfig(block, upstreamServer string) bool {
	return strings.Contains(block, "forward") && strings.Contains(block, upstreamServer)
}

// createRemoteDomainBlock 为 *.remote 域创建一个转发到指定上游的服务器块
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

// removeManagedSection 从 Corefile 内容中移除任何现有的托管部分
func removeManagedSection(content string) string {
	startIdx := strings.Index(content, ManagedSectionStart)
	if startIdx == -1 {
		return content // 未找到托管部分
	}

	endIdx := strings.Index(content, ManagedSectionEnd)
	if endIdx == -1 {
		return content // 格式错误的托管部分，按原样返回
	}

	// 为正确格式化包含结束标记后的换行符
	endIdx += len(ManagedSectionEnd)
	if endIdx < len(content) && content[endIdx] == '\n' {
		endIdx++
	}

	// 返回开始前和结束后的部分
	return content[:startIdx] + content[endIdx:]
}

// updateCorefile 添加或更新 *.remote 域的配置以转发到指定的上游服务器
func updateCorefile(content, upstreamServer string) string {
	// 首先，移除任何现有的托管部分
	contentWithoutManaged := removeManagedSection(content)

	// 在末尾添加新的托管部分
	newBlock := createRemoteDomainBlock(upstreamServer)

	// 确保现有内容和新块之间有一个换行符
	if !strings.HasSuffix(strings.TrimSpace(contentWithoutManaged), "{") &&
		!strings.HasPrefix(newBlock, "\n") {
		newBlock = "\n" + newBlock
	}

	return newBlock + contentWithoutManaged
}

// isManagedSectionPresent 检查托管部分是否已存在于内容中
func isManagedSectionPresent(content string) bool {
	return strings.Contains(content, ManagedSectionStart) && strings.Contains(content, ManagedSectionEnd)
}

// isUpstreamCorrect 检查托管部分中的上游服务器是否与预期服务器匹配
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

// checkCoreDNSConfig 检查 CoreDNS 配置是否已包含我们的上游配置
func checkCoreDNSConfig(content, upstreamServer string) bool {
	// 解析 Corefile 内容
	blocks := parseCorefile(content)

	// 查找处理 *.remote 域的服务器块
	remoteBlock, idx := findServerBlock(blocks, "*.remote")
	if idx == -1 {
		// 未找到 *.remote 的服务器块，因此配置缺失
		return false
	}

	// 检查此块是否具有正确的上游配置
	return hasUpstreamConfig(remoteBlock, upstreamServer)
}

// needsUpdate 确定 Corefile 是否需要更新
func needsUpdate(content, upstreamServer string) bool {
	isManagedSectionPresentStatus := isManagedSectionPresent(content)
	isUpstreamCorrectStatus := isUpstreamCorrect(content, upstreamServer)
	klog.V(4).Infof("Corefile needs update: managed section present: %t, upstream correct: %t",
		isManagedSectionPresentStatus, isUpstreamCorrectStatus)
	return !isManagedSectionPresentStatus || !isUpstreamCorrectStatus
}
