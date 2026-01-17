package k8sclient

import (
	"strings"
	"testing"

	"k8s.io/klog/v2"
)

// 测试用的标准 Corefile
const standardCorefile = `.:53 {
    log
    errors
    health {
       lameduck 5s
    }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
       pods insecure
       fallthrough in-addr.arpa ip6.arpa
       ttl 30
    }
    prometheus :9153
    hosts {
       192.168.39.1 host.minikube.internal
       fallthrough
    }
    forward . /etc/resolv.conf {
       max_concurrent 1000
    }
    cache 30 {
       disable success cluster.local
       disable denial cluster.local
    }
    loop
    reload
    loadbalance
}
`

// 测试用的包含 .:5353 块的 Corefile
const multiBlockCorefile = `.:53 {
    log
    errors
    health {
       lameduck 5s
    }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
       pods insecure
       fallthrough in-addr.arpa ip6.arpa
       ttl 30
    }
    prometheus :9153
    hosts {
       192.168.39.1 host.minikube.internal
       fallthrough
    }
    forward . /etc/resolv.conf {
       max_concurrent 1000
    }
    cache 30 {
       disable success cluster.local
       disable denial cluster.local
    }
    loop
    reload
    loadbalance
}

.:5353 {
    log
    errors
    health {
       lameduck 5s
    }
    ready
    kubernetes cluster.local in-addr.arpa ip6.arpa {
       pods insecure
       fallthrough in-addr.arpa ip6.arpa
       ttl 30
    }
    prometheus :9154
    forward . /etc/resolv.conf {
       max_concurrent 1000
    }
    cache 30
    loop
    reload
    loadbalance
}
`

func init() {
	klog.InitFlags(nil)
}

func TestFindBlockEnd(t *testing.T) {
	tests := []struct {
		name        string
		corefile    string
		prefix      string
		exactMatch  bool
		expectedIdx int
	}{
		{
			name:        "Find .:53 block end - exact match",
			corefile:    standardCorefile,
			prefix:      ".:53",
			exactMatch:  true,
			expectedIdx: 27, // 最后一个 } 所在的行
		},
		{
			name:        "Find .:53 block end - not exact",
			corefile:    standardCorefile,
			prefix:      ".:53",
			exactMatch:  false,
			expectedIdx: 27,
		},
		{
			name:        "Find .:53 block end - exclude .:5353",
			corefile:    multiBlockCorefile,
			prefix:      ".:53",
			exactMatch:  true,
			expectedIdx: 27,
		},
		{
			name:        "Find .:5353 block end",
			corefile:    multiBlockCorefile,
			prefix:      ".:5353",
			exactMatch:  false,
			expectedIdx: 49,
		},
		{
			name:        "Find non-existent block",
			corefile:    standardCorefile,
			prefix:      ".:9999",
			exactMatch:  false,
			expectedIdx: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.corefile, "\n")
			result := findBlockEnd(lines, tt.prefix, tt.exactMatch)

			if result != tt.expectedIdx {
				t.Errorf("findBlockEnd() = %v, want %v", result, tt.expectedIdx)
			}

			if klog.V(2).Enabled() {
				klog.V(2).Infof("Test completed: name=%s, corefileLines=%d, foundBlockEnd=%d, line=%s", tt.name, len(lines), result, lines[result])
			}
		})
	}
}

func TestFindFirstBlockEnd(t *testing.T) {
	tests := []struct {
		name        string
		corefile    string
		expectedIdx int
	}{
		{
			name:        "Find first block end in standard corefile",
			corefile:    standardCorefile,
			expectedIdx: 27,
		},
		{
			name:        "Find first block end in multi-block corefile",
			corefile:    multiBlockCorefile,
			expectedIdx: 27,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.corefile, "\n")
			result := findFirstBlockEnd(lines)

			if result != tt.expectedIdx {
				t.Errorf("findFirstBlockEnd() = %v, want %v", result, tt.expectedIdx)
			}

			if klog.V(2).Enabled() {
				klog.V(2).Infof("Test completed: name=%s, foundBlockEnd=%d, line=%s", tt.name, result, lines[result])
			}
		})
	}
}

func TestInsertAfterBlock(t *testing.T) {
	const insertContent = "# test insert\n"
	
	corefileSection := func(builder *strings.Builder) {
		builder.WriteString(insertContent)
	}

	tests := []struct {
		name           string
		corefile       string
		insertIdx      int
		shouldContain  string
	}{
		{
			name:          "Insert after .:53 block",
			corefile:      standardCorefile,
			insertIdx:     27,
			shouldContain: insertContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.corefile, "\n")
			var builder strings.Builder

			if klog.V(2).Enabled() {
				klog.V(2).Infof("=== %s ===\nBefore insertion:\n%s", tt.name, tt.corefile)
			}

			insertAfterBlock(lines, tt.insertIdx, &builder, corefileSection)
			result := builder.String()

			if !strings.Contains(result, tt.shouldContain) {
				t.Errorf("insertAfterBlock() result should contain %q", tt.shouldContain)
			}

			// Verify that the content is inserted after the specified line
			resultLines := strings.Split(result, "\n")
			found := false
			for i, line := range resultLines {
				if i > tt.insertIdx && strings.Contains(line, "test insert") {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("insertAfterBlock() content not found after line %d", tt.insertIdx)
			}

			if klog.V(2).Enabled() {
				klog.V(2).Infof("After insertion:\n%s", result)
			}
		})
	}
}

func TestInsertManagedBlockPriority(t *testing.T) {
	const podIP = "10.244.1.100"
	const startMarkerLine1 = "# k8s-cross-cluster start. The lines below are managed by coredns-config-manager automatically."
	const startMarkerLine2 = "# You are not supposed to edit it."
	const endMarker = "# k8s-cross-cluster end."

	corefileSection := func(builder *strings.Builder) {
		builder.WriteString(startMarkerLine1 + "\n")
		builder.WriteString(startMarkerLine2 + "\n")
		builder.WriteString("## Forward to coredns-config-manager DNS server.\n")
		builder.WriteString("forward . " + podIP + "\n")
		builder.WriteString(endMarker + "\n")
	}

	tests := []struct {
		name       string
		corefile   string
		target     string
	}{
		{
			name:     "Insert into .:53 block",
			corefile: standardCorefile,
			target:   ".:53",
		},
		{
			name:     "Insert into .:5353 block when .:53 unavailable",
			corefile: multiBlockCorefile,
			target:   ".:5353",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.corefile, "\n")
			var builder strings.Builder

			if klog.V(2).Enabled() {
				klog.V(2).Infof("=== %s ===\nBefore insertion:\n%s", tt.name, tt.corefile)
			}

			var insertIdx int
			if tt.target == ".:53" {
				insertIdx = findBlockEnd(lines, ".:53", true)
			} else {
				insertIdx = findBlockEnd(lines, tt.target, false)
			}

			if insertIdx >= 0 {
				insertAfterBlock(lines, insertIdx, &builder, corefileSection)
			} else {
				// Fallback to first block
				insertIdx = findFirstBlockEnd(lines)
				insertAfterBlock(lines, insertIdx, &builder, corefileSection)
			}

			result := builder.String()

			if !strings.Contains(result, startMarkerLine1) {
				t.Errorf("Result should contain start marker")
			}

			if !strings.Contains(result, "forward . "+podIP) {
				t.Errorf("Result should contain forward line with IP %s", podIP)
			}

			if !strings.Contains(result, endMarker) {
				t.Errorf("Result should contain end marker")
			}

			if klog.V(2).Enabled() {
				klog.V(2).Infof("After insertion:\n%s", result)
			}
		})
	}
}

func TestCorefileBlockParsing(t *testing.T) {
	tests := []struct {
		name           string
		corefile       string
		blocksToFind   []string
	}{
		{
			name: "Parse standard corefile blocks",
			corefile: standardCorefile,
			blocksToFind: []string{".:53"},
		},
		{
			name: "Parse multi-block corefile",
			corefile: multiBlockCorefile,
			blocksToFind: []string{".:53", ".:5353"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := strings.Split(tt.corefile, "\n")

			if klog.V(2).Enabled() {
				klog.V(2).Infof("=== %s ===\nParsing corefile:\n%s", tt.name, tt.corefile)
			}

			for _, block := range tt.blocksToFind {
				var insertIdx int
				if block == ".:53" {
					insertIdx = findBlockEnd(lines, block, true)
				} else {
					insertIdx = findBlockEnd(lines, block, false)
				}

				if insertIdx < 0 {
					t.Errorf("Block %s not found", block)
					continue
				}

				if klog.V(2).Enabled() {
					klog.V(2).Infof("Block %s ends at line %d: %s", block, insertIdx, lines[insertIdx])
				}

				// Find block start for verification
				blockStart := -1
				for i, line := range lines {
					if strings.Contains(line, block+" {") {
						blockStart = i
						break
					}
				}

				if blockStart >= 0 && klog.V(2).Enabled() {
					blockContent := strings.Join(lines[blockStart:insertIdx+1], "\n")
					klog.V(2).Infof("Block %s content:\n%s", block, blockContent)
				}
			}
		})
	}
}

func TestRemoveManagedBlockFromCorefile(t *testing.T) {
	const startMarkerLine1 = "# k8s-cross-cluster start. The lines below are managed by coredns-config-manager automatically."
	const startMarkerLine2 = "# You are not supposed to edit it."
	const endMarker = "# k8s-cross-cluster end."

	tests := []struct {
		name           string
		corefile       string
		expectedResult string
		expectedRemoved bool
	}{
		{
			name: "Remove managed block successfully",
			corefile: standardCorefile + "\n" + startMarkerLine1 + "\n" + startMarkerLine2 + "\nforward . 10.244.1.100\n" + endMarker + "\n",
			expectedResult: standardCorefile + "\n\n",
			expectedRemoved: true,
		},
		{
			name: "No managed block present",
			corefile: standardCorefile,
			expectedResult: standardCorefile,
			expectedRemoved: false,
		},
		{
			name: "Only start marker without end marker",
			corefile: standardCorefile + "\n" + startMarkerLine1 + "\nforward . 10.244.1.100\n",
			expectedResult: standardCorefile + "\n" + startMarkerLine1 + "\nforward . 10.244.1.100\n",
			expectedRemoved: false,
		},
		{
			name: "Only end marker without start marker",
			corefile: standardCorefile + "\n" + endMarker + "\n",
			expectedResult: standardCorefile + "\n" + endMarker + "\n",
			expectedRemoved: false,
		},
		{
			name: "Empty corefile",
			corefile: "",
			expectedResult: "",
			expectedRemoved: false,
		},
		{
			name: "Managed block in the middle",
			corefile: "header line\n" + startMarkerLine1 + "\nforward . 10.244.1.100\n" + endMarker + "\ntrailer line\n",
			expectedResult: "header line\n\ntrailer line\n",
			expectedRemoved: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, removed := removeManagedBlockFromCorefile(tt.corefile)

			if removed != tt.expectedRemoved {
				t.Errorf("removeManagedBlockFromCorefile() removed = %v, want %v", removed, tt.expectedRemoved)
			}

			if result != tt.expectedResult {
				t.Errorf("removeManagedBlockFromCorefile() result mismatch\ngot:\n%s\nwant:\n%s", result, tt.expectedResult)
			}

			if klog.V(2).Enabled() {
				klog.V(2).Infof("=== %s ===\nInput:\n%s\nOutput:\n%s\nRemoved: %v", tt.name, tt.corefile, result, removed)
			}
		})
	}
}