package dnsserver

import (
	"fmt"
	"net"
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
	records map[string][]DNSRecord // key: domain name
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
// GetAllRecords 获取所有DNS记录，用于调试
func (s *DNSServer) GetAllRecords() map[string][]DNSRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string][]DNSRecord)
	for name, records := range s.records {
		// 硬拷贝，以免影响 dns 服务器状态
		result[name] = append([]DNSRecord{}, records...)

		// 日志输出所有记录
		for _, record := range records {
			klog.Infof("[DEBUG] DNS Record - Domain: %s, Type: %s (%d), TTL: %d, Value: %s",
				name, dns.TypeToString[record.Type], record.Type, record.TTL, record.Value)
		}
	}
	return result
}

// handleDNSRequest 处理DNS请求
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
			for _, record := range records {
				if record.Type == qtype {
					rr, err := dns.NewRR(fmt.Sprintf("%s %d IN %s %s", domain, record.TTL, dns.TypeToString[record.Type], record.Value))
					if err != nil {
						klog.Errorf("Failed to create DNS RR: %v", err)
						continue
					}
					msg.Answer = append(msg.Answer, rr)
				}
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
