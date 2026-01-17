package k8sclient

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const CaddyConfigMapName = "caddy-config"
const CaddyConfigKey = "Caddyfile"

const coreDNSNamespace = "kube-system"
const CoreDNSConfigMapName = "coredns"
const CoreDNSConfigKey = "Corefile"

const startMarkerLine1 = "# k8s-cross-cluster start. The lines below are managed by coredns-config-manager automatically."
const startMarkerLine2 = "# You are not supposed to edit it."
const endMarker = "# k8s-cross-cluster end."
const forwardCommentLine = "## Forward to coredns-config-manager DNS server."

// UpdateCaddyConfigMap creates or updates the ConfigMap with Caddy configuration
func UpdateCaddyConfigMap(clientset kubernetes.Interface, namespaceProvided *string, caddyConfig string) error {
	ctx := context.Background()
	ns := GetCurrentNamespaceOrProvided(namespaceProvided)

	configMaps := clientset.CoreV1().ConfigMaps(ns)

	// Check if ConfigMap exists
	existingCM, err := configMaps.Get(ctx, CaddyConfigMapName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// ConfigMap does not exist, create it
			newCM := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      CaddyConfigMapName,
					Namespace: ns,
				},
				Data: map[string]string{
					CaddyConfigKey: caddyConfig,
				},
			}
			_, err = configMaps.Create(ctx, newCM, metav1.CreateOptions{})
			if err != nil {
				klog.Errorf("Failed to create ConfigMap %s: %v", CaddyConfigMapName, err)
				return err
			}
			klog.Infof("Created ConfigMap %s successfully", CaddyConfigMapName)
			return nil
		}
		klog.Errorf("Failed to get ConfigMap %s: %v", CaddyConfigMapName, err)
		return err
	}

	// ConfigMap exists, update it
	if existingCM.Data == nil {
		existingCM.Data = make(map[string]string)
	}
	existingCM.Data[CaddyConfigKey] = caddyConfig

	_, err = configMaps.Update(ctx, existingCM, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update ConfigMap %s: %v", CaddyConfigMapName, err)
		return err
	}
	klog.Infof("Updated ConfigMap %s successfully", CaddyConfigMapName)
	return nil
}

// UpdateCoreDNSConfigMap updates the CoreDNS ConfigMap to add forward configuration for our DNS server
func UpdateCoreDNSConfigMap(clientset kubernetes.Interface, podIP string) error {
	ctx := context.Background()

	configMaps := clientset.CoreV1().ConfigMaps(coreDNSNamespace)

	// Get existing CoreDNS ConfigMap
	existingCM, err := configMaps.Get(ctx, CoreDNSConfigMapName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get CoreDNS ConfigMap: %v", err)
		return err
	}

	// Get existing Corefile content
	corefile := existingCM.Data[CoreDNSConfigKey]
	if corefile == "" {
		klog.Errorf("Corefile not found in ConfigMap %s", CoreDNSConfigMapName)
		return fmt.Errorf("Corefile not found")
	}

	// Define markers for the managed block
	forwardLine := fmt.Sprintf("forward . %s", podIP)

	// Check if the managed block exists
	startIdx := strings.Index(corefile, startMarkerLine1)
	endIdx := strings.Index(corefile, endMarker)

	var corefileSectionBuilder strings.Builder

	// Define closure to write the managed block content
	corefileSection := func(builder *strings.Builder) {
		builder.WriteString(startMarkerLine1 + "\n")
		builder.WriteString(startMarkerLine2 + "\n")
		builder.WriteString(forwardCommentLine + "\n")
		builder.WriteString(forwardLine + "\n")
		builder.WriteString(endMarker + "\n")
	}

	if startIdx >= 0 && endIdx >= 0 && startIdx < endIdx {
		// Managed block exists, update it
		corefileSectionBuilder.WriteString(corefile[:startIdx])
		corefileSection(&corefileSectionBuilder)
		corefileSectionBuilder.WriteString(corefile[endIdx+len(endMarker):])
		klog.Infof("Updated existing managed block with new IP: %s", podIP)
	} else {
		// Managed block does not exist, insert it with priority
		lines := strings.Split(corefile, "\n")
		inserted := false

		// Try to find .:53 block end and insert there
		insertIdx := findBlockEnd(lines, ".:53", true)
		if insertIdx >= 0 {
			insertAfterBlock(lines, insertIdx, &corefileSectionBuilder, corefileSection)
			klog.Infof("Inserted new managed block after .:53 block at line %d with IP: %s", insertIdx, podIP)
			inserted = true
		}

		// Fallback: Try to find .:5353 block end and insert there
		if !inserted {
			insertIdx = findBlockEnd(lines, ".:5353", false)
			if insertIdx >= 0 {
				insertAfterBlock(lines, insertIdx, &corefileSectionBuilder, corefileSection)
				klog.Infof("Inserted new managed block after .:5353 block at line %d with IP: %s", insertIdx, podIP)
				inserted = true
			}
		}

		// Fallback: Try to find first block end and insert there
		if !inserted {
			insertIdx = findFirstBlockEnd(lines)
			if insertIdx >= 0 {
				insertAfterBlock(lines, insertIdx, &corefileSectionBuilder, corefileSection)
				klog.Warningf("Inserted new managed block after first block at line %d with IP: %s", insertIdx, podIP)
				inserted = true
			}
		}

		// Fallback: Append at the end// caddy
		if !inserted {
			for _, line := range lines {
				corefileSectionBuilder.WriteString(line + "\n")
			}
			corefileSection(&corefileSectionBuilder)
			klog.Warningf("Appended new managed block at the end with IP: %s", podIP)
		}
	}

	// Update ConfigMap
	if existingCM.Data == nil {
		existingCM.Data = make(map[string]string)
	}
	existingCM.Data[CoreDNSConfigKey] = corefileSectionBuilder.String()

	_, err = configMaps.Update(ctx, existingCM, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update CoreDNS ConfigMap: %v", err)
		return err
	}

	klog.Infof("Successfully updated CoreDNS ConfigMap with forward configuration for %s", podIP)
	return nil
}

// findBlockEnd finds the end line index of a block that starts with the given prefix
// exactMatch determines if we need an exact match (for .:53 to exclude .:5353)
func findBlockEnd(lines []string, prefix string, exactMatch bool) int {
	blockStack := 0
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.Contains(trimmedLine, "{") && strings.Contains(trimmedLine, ".:") {
			if exactMatch {
				// exactMatch：行必须以 prefix 开头，后跟空格和内容
				if strings.HasPrefix(trimmedLine, prefix+" ") || trimmedLine == prefix+" {" {
					blockStack = 1
				}
			} else {
				if strings.Contains(trimmedLine, prefix) {
					blockStack = 1
				}
			}
			if blockStack == 1 {
				continue
			}
		}
		if blockStack > 0 {
			if strings.Contains(trimmedLine, "{") {
				blockStack++
			}
			if strings.Contains(trimmedLine, "}") {
				blockStack--
				if blockStack == 0 {
					return i
				}
			}
		}
	}
	return -1
}

// findFirstBlockEnd finds the end line index of the first block
func findFirstBlockEnd(lines []string) int {
	blockStack := 0
	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.Contains(trimmedLine, "{") && strings.Contains(trimmedLine, ".:") {
			blockStack = 1
			continue
		}
		if blockStack > 0 {
			if strings.Contains(trimmedLine, "{") {
				blockStack++
			}
			if strings.Contains(trimmedLine, "}") {
				blockStack--
				if blockStack == 0 {
					return i
				}
			}
		}
	}
	return -1
}

// insertAfterBlock inserts content after a specific line index
func insertAfterBlock(lines []string, insertIdx int, builder *strings.Builder, corefileSection func(*strings.Builder)) {
	for i, line := range lines {
		builder.WriteString(line + "\n")
		if i == insertIdx {
			corefileSection(builder)
		}
	}
}

// RemoveManagedBlock removes the managed block from CoreDNS ConfigMap
func removeManagedBlockFromCorefile(corefile string) (string, bool) {
	// Check if the managed block exists
	startIdx := strings.Index(corefile, startMarkerLine1)
	endIdx := strings.Index(corefile, endMarker)

	if startIdx < 0 || endIdx < 0 || startIdx > endIdx {
		return corefile, false
	}

	// Remove the managed block
	newCorefile := corefile[:startIdx] + corefile[endIdx+len(endMarker):]
	return newCorefile, true
}

func RemoveManagedBlock(clientset kubernetes.Interface) error {
	ctx := context.Background()
	configMaps := clientset.CoreV1().ConfigMaps(coreDNSNamespace)

	// Get existing CoreDNS ConfigMap
	existingCM, err := configMaps.Get(ctx, CoreDNSConfigMapName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get CoreDNS ConfigMap: %v", err)
		return err
	}

	// Get existing Corefile content
	corefile := existingCM.Data[CoreDNSConfigKey]
	if corefile == "" {
		klog.Warningf("Corefile not found in ConfigMap %s", CoreDNSConfigMapName)
		return nil
	}

	// Remove managed block from corefile content
	newCorefile, removed := removeManagedBlockFromCorefile(corefile)
	if !removed {
		klog.Info("No managed block found, nothing to remove")
		return nil
	}

	// Update ConfigMap
	if existingCM.Data == nil {
		existingCM.Data = make(map[string]string)
	}
	existingCM.Data[CoreDNSConfigKey] = newCorefile

	_, err = configMaps.Update(ctx, existingCM, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("Failed to update CoreDNS ConfigMap: %v", err)
		return err
	}

	klog.Info("Successfully removed managed block from CoreDNS ConfigMap")
	return nil
}
