package dnsserver

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	dns "codeberg.org/miekg/dns"
	"k8s.io/klog/v2"
)

// Server 表示一个DNS服务器实例
type Server struct {
	addr     string
	server   *dns.Server
	mu       sync.RWMutex
	records  map[string][]dns.RR // 域名 -> DNS记录列表

	// 用于热加载的通道
	updateChan chan *RecordUpdate
	stopChan   chan struct{}
}

// RecordUpdate 表示DNS记录更新
type RecordUpdate struct {
	Action  UpdateAction
	Domain  string
	Records []dns.RR
}

// UpdateAction 表示更新操作类型
type UpdateAction int

const (
	// AddRecords 添加记录
	AddRecords UpdateAction = iota
	// RemoveRecords 移除记录
	RemoveRecords
	// ReplaceRecords 替换记录
	ReplaceRecords
	// ClearAll 清除所有记录
	ClearAll
)

// NewServer 创建一个新的DNS服务器
func NewServer(addr string) *Server {
	s := &Server{
		addr:       addr,
		records:    make(map[string][]dns.RR),
		updateChan: make(chan *RecordUpdate, 100),
		stopChan:   make(chan struct{}),
	}

	return s
}

// Start 启动DNS服务器
func (s *Server) Start() error {
	// 创建DNS服务器
	s.server = &dns.Server{
		Addr: s.addr,
		Net:  "udp",
		Handler: dns.HandlerFunc(func(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) {
			s.handleRequest(ctx, w, r)
		}),
	}

	// 启动更新处理器
	go s.processUpdates()

	// 启动DNS服务器
	klog.Infof("Starting DNS server on %s", s.addr)
	go func() {
		if err := s.server.ListenAndServe(); err != nil {
			klog.Errorf("DNS server failed: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 测试服务器是否启动
	if err := s.testConnection(); err != nil {
		return fmt.Errorf("DNS server failed to start: %v", err)
	}

	return nil
}

// Stop 停止DNS服务器
func (s *Server) Stop() {
	close(s.stopChan)
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}
}

// GetIP 获取DNS服务器的IP地址
func (s *Server) GetIP() (string, error) {
	host, port, err := net.SplitHostPort(s.addr)
	if err != nil {
		// 如果没有端口，尝试添加默认端口
		host = s.addr
		port = "53"
	}

	if host == "" || host == "0.0.0.0" || host == "::" {
		// 获取本地IP
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return "", fmt.Errorf("failed to get interface addresses: %v", err)
		}
		// 返回获取到的第一个非回环IP
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return net.JoinHostPort(ipnet.IP.String(), port), nil
				}
			}
		}
		return "", fmt.Errorf("no non-loopback IPv4 address found")
	}

	// 解析主机名
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", fmt.Errorf("failed to lookup IP for %s: %v", host, err)
	}

	for _, ip := range ips {
		if ip.To4() != nil {
			return net.JoinHostPort(ip.String(), port), nil
		}
	}

	return "", fmt.Errorf("no IPv4 address found for %s", host)
}

// AddRecords 添加DNS记录
func (s *Server) AddRecords(domain string, records []dns.RR) {
	s.updateChan <- &RecordUpdate{
		Action:  AddRecords,
		Domain:  domain,
		Records: records,
	}
}

// RemoveRecords 移除DNS记录
func (s *Server) RemoveRecords(domain string) {
	s.updateChan <- &RecordUpdate{
		Action: RemoveRecords,
		Domain: domain,
	}
}

// ReplaceRecords 替换DNS记录
func (s *Server) ReplaceRecords(domain string, records []dns.RR) {
	s.updateChan <- &RecordUpdate{
		Action:  ReplaceRecords,
		Domain:  domain,
		Records: records,
	}
}

// ClearAllRecords 清除所有DNS记录
func (s *Server) ClearAllRecords() {
	s.updateChan <- &RecordUpdate{
		Action: ClearAll,
	}
}

// GetRecords 获取指定域名的DNS记录
func (s *Server) GetRecords(domain string) []dns.RR {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if records, exists := s.records[domain]; exists {
		// 返回副本
		result := make([]dns.RR, len(records))
		copy(result, records)
		return result
	}

	return nil
}

// GetAllRecords 获取所有DNS记录
func (s *Server) GetAllRecords() map[string][]dns.RR {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回深拷贝
	result := make(map[string][]dns.RR)
	for domain, records := range s.records {
		recordsCopy := make([]dns.RR, len(records))
		copy(recordsCopy, records)
		result[domain] = recordsCopy
	}

	return result
}

// handleRequest 处理DNS请求
func (s *Server) handleRequest(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	// 设置回复消息ID
	m.ID = r.ID
	m.Response = true
	m.Authoritative = true

	// 处理每个查询
	for _, q := range r.Question {
		// 从RR中提取域名和类型
		domain := q.Header().Name
		qtype := dns.RRToType(q)

		s.mu.RLock()
		records, exists := s.records[domain]
		s.mu.RUnlock()

		if exists {
			// 过滤出请求类型的记录
			for _, rr := range records {
				if dns.RRToType(rr) == qtype || qtype == dns.TypeANY {
					m.Answer = append(m.Answer, rr)
				}
			}
		}

		// 如果没有找到记录，尝试通配符匹配
		if len(m.Answer) == 0 {
			// 简单的通配符匹配：*.example.com
			wildcardDomain := "*." + domain
			s.mu.RLock()
			wildcardRecords, wildcardExists := s.records[wildcardDomain]
			s.mu.RUnlock()

			if wildcardExists {
				for _, rr := range wildcardRecords {
					if dns.RRToType(rr) == qtype || qtype == dns.TypeANY {
						// 复制记录并修改域名为实际查询的域名
						rrCopy := rr.(interface{ Clone() dns.RR }).Clone()
						header := rrCopy.Header()
						header.Name = domain
						m.Answer = append(m.Answer, rrCopy)
					}
				}
			}
		}
	}

	// 如果没有找到记录，返回NXDOMAIN
	if len(m.Answer) == 0 {
		m.Rcode = dns.RcodeNameError
	}

	// 设置问题部分
	m.Question = r.Question

	// 打包消息
	if err := m.Pack(); err != nil {
		klog.Errorf("Failed to pack DNS message: %v", err)
		return
	}

	// 写入响应
	if _, err := w.Write(m.Data); err != nil {
		klog.Errorf("Failed to write DNS response: %v", err)
	}
}

// processUpdates 处理记录更新
func (s *Server) processUpdates() {
	for {
		select {
		case <-s.stopChan:
			return
		case update := <-s.updateChan:
			s.applyUpdate(update)
		}
	}
}

// applyUpdate 应用记录更新
func (s *Server) applyUpdate(update *RecordUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch update.Action {
	case AddRecords:
		if existing, exists := s.records[update.Domain]; exists {
			s.records[update.Domain] = append(existing, update.Records...)
		} else {
			s.records[update.Domain] = update.Records
		}
		klog.Infof("Added %d records for domain %s", len(update.Records), update.Domain)

	case RemoveRecords:
		delete(s.records, update.Domain)
		klog.Infof("Removed all records for domain %s", update.Domain)

	case ReplaceRecords:
		s.records[update.Domain] = update.Records
		klog.Infof("Replaced records for domain %s with %d records", update.Domain, len(update.Records))

	case ClearAll:
		s.records = make(map[string][]dns.RR)
		klog.Info("Cleared all DNS records")
	}
}

// testConnection 测试服务器连接
func (s *Server) testConnection() error {
	// 创建一个简单的DNS客户端来测试连接
	c := new(dns.Client)
	m := dns.NewMsg("test.local.", dns.TypeA)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, _, err := c.Exchange(ctx, m, "udp", s.addr)
	// 我们期望超时或拒绝连接，但不应该是其他错误
	if err != nil {
		// 检查是否是连接被拒绝（服务器已启动但未处理我们的测试查询）
		if opErr, ok := err.(*net.OpError); ok {
			if opErr.Op == "read" || opErr.Op == "write" {
				// 连接被拒绝意味着服务器正在运行但未监听
				return nil
			}
		}
		// 超时也是可以接受的
		if err == context.DeadlineExceeded {
			return nil
		}
		return err
	}

	return nil
}

// CreateARecord 创建A记录
func CreateARecord(domain string, ip string, ttl uint32) dns.RR {
	// 使用New函数创建记录
	record, err := dns.New(fmt.Sprintf("%s %d IN A %s", domain, ttl, ip))
	if err != nil {
		klog.Errorf("Failed to create A record: %v", err)
		return nil
	}
	return record
}

// CreateCNAMERecord 创建CNAME记录
func CreateCNAMERecord(domain string, target string, ttl uint32) dns.RR {
	// 使用New函数创建记录
	record, err := dns.New(fmt.Sprintf("%s %d IN CNAME %s", domain, ttl, target))
	if err != nil {
		klog.Errorf("Failed to create CNAME record: %v", err)
		return nil
	}
	return record
}

// CreateTXTRecord 创建TXT记录
func CreateTXTRecord(domain string, text string, ttl uint32) dns.RR {
	// 使用New函数创建记录
	record, err := dns.New(fmt.Sprintf("%s %d IN TXT \"%s\"", domain, ttl, text))
	if err != nil {
		klog.Errorf("Failed to create TXT record: %v", err)
		return nil
	}
	return record
}

// CreateWildcardRecord 创建通配符记录
func CreateWildcardRecord(pattern string, rr dns.RR) dns.RR {
	// 复制记录并修改域名为通配符模式
	rrCopy := rr.(interface{ Clone() dns.RR }).Clone()
	header := rrCopy.Header()
	header.Name = pattern
	return rrCopy
}
