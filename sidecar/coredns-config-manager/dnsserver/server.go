package dnsserver

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
	"k8s.io/klog/v2"
)

// DNSRecord 表示一条DNS记录
type DNSRecord struct {
	Name  string
	Type  uint16
	TTL   uint32
	Value string
}

// DNSServer DNS服务器
type DNSServer struct {
	server  *dns.Server
	records map[string][]DNSRecord // 键: 域名
	mu      sync.RWMutex
	addr    string
}

// NewDNSServer 创建一个新的DNS服务器实例
func NewDNSServer(addr string) *DNSServer {
	return &DNSServer{
		records: make(map[string][]DNSRecord),
		addr:    addr,
	}
}

// AddRecord 添加DNS记录
func (s *DNSServer) AddRecord(name string, recordType uint16, ttl uint32, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records[name] = append(s.records[name], DNSRecord{
		Name:  name,
		Type:  recordType,
		TTL:   ttl,
		Value: value,
	})

	klog.Infof("Added DNS record: %s %d %s %s", name, recordType, value, "success")
}

// RemoveRecords 移除指定域名的所有记录
func (s *DNSServer) RemoveRecords(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.records, name)
	klog.Infof("Removed DNS records for: %s", name)
}

// UpdateRecords 更新指定域名的记录
func (s *DNSServer) UpdateRecords(name string, records []DNSRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records[name] = records
	klog.Infof("Updated DNS records for: %s, count: %d", name, len(records))
}

// GetRecords 获取指定域名的所有记录
func (s *DNSServer) GetRecords(name string) []DNSRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if records, ok := s.records[name]; ok {
		return records
	}
	return nil
}

// handleDNSRequest 处理DNS请求
// buildAnswersForQuery 为域名和查询类型构建DNS资源记录
func buildAnswersForQuery(domain string, qtype uint16, records []DNSRecord) []dns.RR {
	var answers []dns.RR
	for _, record := range records {
		if record.Type == qtype {
			rr, err := dns.NewRR(fmt.Sprintf("%s %d IN %s %s", domain, record.TTL, dns.TypeToString[record.Type], record.Value))
			if err != nil {
				klog.Errorf("Failed to create DNS RR: %v", err)
				continue
			}
			answers = append(answers, rr)
		}
	}
	return answers
}

// matchWildcardPattern 检查域名是否匹配通配符模式
// 例如: *.*.foo.com 匹配 a.b.foo.com, a.*.foo.com 匹配 a.b.foo.com
// 通配符 * 只能匹配单级域名部分
func matchWildcardPattern(domain, pattern string) bool {
	// 标准化：去除根域点号
	domain = strings.TrimSuffix(domain, ".")
	pattern = strings.TrimSuffix(pattern, ".")

	// 分割成各个子域
	domainParts := strings.Split(domain, ".")
	patternParts := strings.Split(pattern, ".")

	// 检验子域数量一致性
	if len(domainParts) != len(patternParts) {
		return false
	}

	// 逐部分比较
	for i := 0; i < len(domainParts); i++ {
		if patternParts[i] == "*" {
			// 通配符匹配任意单个部分
			continue
		}
		// 部分不匹配
		if patternParts[i] != domainParts[i] {
			return false
		}
	}

	return true
}

// findWildcardMatchingRecords 查找所有匹配指定域名的通配符记录
// 返回所有匹配的记录，key为匹配的通配符模式
func findWildcardMatchingRecords(domain string, records map[string][]DNSRecord) map[string][]DNSRecord {
	matches := make(map[string][]DNSRecord)

	for pattern, recordList := range records {
		// 跳过非通配符模式（不包含 * 的模式）
		if !strings.Contains(pattern, "*") {
			continue
		}

		// 检查是否匹配
		if matchWildcardPattern(domain, pattern) {
			// 硬拷贝，以免影响原数据
			matches[pattern] = append([]DNSRecord{}, recordList...)
			klog.Infof("Wildcard match found: domain %s matches pattern %s", domain, pattern)
		}
	}

	return matches
}

func (s *DNSServer) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, question := range r.Question {
		domain := question.Name
		qtype := question.Qtype

		klog.V(4).Infof("DNS query: %s type %d from %s", domain, qtype, w.RemoteAddr())

		if records, ok := s.records[domain]; ok {
			// 精确匹配
			answers := buildAnswersForQuery(domain, qtype, records)
			msg.Answer = append(msg.Answer, answers...)
		} else {
			// 尝试通配符匹配
			klog.Infof("handleDNSRequest: Trying wildcard matching for %s", domain)
			matches := findWildcardMatchingRecords(domain, s.records)

			for _, records := range matches {
				answers := buildAnswersForQuery(domain, qtype, records)
				msg.Answer = append(msg.Answer, answers...)
				klog.V(4).Infof("DNS wildcard matched: %s -> records from wildcard pattern", domain)
			}
		}
	}

	if err := w.WriteMsg(msg); err != nil {
		klog.Errorf("Failed to send DNS response: %v", err)
	}
}

// Start 启动DNS服务器
func (s *DNSServer) Start() error {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", s.handleDNSRequest)

	s.server = &dns.Server{
		Addr:    s.addr,
		Net:     "udp",
		Handler: mux,
	}

	go func() {
		klog.Infof("Starting DNS server on %s", s.addr)
		if err := s.server.ListenAndServe(); err != nil {
			klog.Errorf("DNS server error: %v", err)
		}
	}()

	// 等待服务器启动
	for i := 0; i < 10; i++ {
		conn, err := net.Dial("udp", s.addr)
		if err == nil {
			conn.Close()
			klog.Infof("DNS server successfully started on %s", s.addr)
			return nil
		}
	}

	return fmt.Errorf("DNS server failed to start on %s", s.addr)
}

// Stop 停止DNS服务器
func (s *DNSServer) Stop() error {
	if s.server != nil {
		klog.Infof("Stopping DNS server on %s", s.addr)
		return s.server.Shutdown()
	}
	return nil
}

// GetAddr 获取DNS服务器监听地址
func (s *DNSServer) GetAddr() string {
	return s.addr
}
