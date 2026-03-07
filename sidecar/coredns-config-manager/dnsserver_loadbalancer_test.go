package main

import (
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"

	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
)

// TestDNSServer_WithLoadBalancerHandler_ClustersetRemote 测试 dnsserver 在注册了
// LoadBalancer.HandleQuery 作为 QueryHandler 的情况下能够正常识别 .clusterset.remote 域名查询
func TestDNSServer_WithLoadBalancerHandler_ClustersetRemote(t *testing.T) {
	// 1. 创建 DNS 服务器
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:15370")
	dnsSrv.Start()
	defer dnsSrv.Stop()

	time.Sleep(100 * time.Millisecond)

	// 2. 创建 ServiceDiscovery 并预填充服务缓存
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
			Count: 1,
		},
	}

	// 3. 创建 LoadBalancer
	lb := NewLoadBalancer(sd, dnsSrv, nil)

	// 4. 将 LoadBalancer.HandleQuery 注册为 DNS 服务器的 QueryHandler
	dnsSrv.RegisterQueryHandler(lb.HandleQuery)

	// 5. 发起 DNS 查询测试
	c := new(dns.Client)

	// 测试: 查询 .clusterset.remote 域名
	m := new(dns.Msg)
	m.SetQuestion("myapp.default.svc.clusterset.remote.", dns.TypeA)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, dnsSrv.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r.Rcode)
	assert.Len(t, r.Answer, 1)
	assert.Equal(t, "10.96.1.10", r.Answer[0].(*dns.A).A.String())
}

// TestDNSServer_WithLoadBalancerHandler_ClustersetRemote_MultipleClusters 测试多集群场景
func TestDNSServer_WithLoadBalancerHandler_ClustersetRemote_MultipleClusters(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:15371")
	dnsSrv.Start()
	defer dnsSrv.Stop()

	time.Sleep(100 * time.Millisecond)

	// 预填充多个集群的服务
	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
			Count: 1,
		},
		"cluster2": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.2.10"},
				},
			},
			Count: 1,
		},
		"cluster3": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.3.10"},
				},
			},
			Count: 1,
		},
	}

	lb := NewLoadBalancer(sd, dnsSrv, nil)
	dnsSrv.RegisterQueryHandler(lb.HandleQuery)

	c := new(dns.Client)

	// 测试轮询负载均衡
	ips := make(map[string]bool)
	for i := 0; i < 6; i++ {
		m := new(dns.Msg)
		m.SetQuestion("myapp.default.svc.clusterset.remote.", dns.TypeA)
		m.RecursionDesired = false

		r, _, err := c.Exchange(m, dnsSrv.GetAddr())
		assert.NoError(t, err)
		assert.Equal(t, dns.RcodeSuccess, r.Rcode)
		assert.Len(t, r.Answer, 1)

		ip := r.Answer[0].(*dns.A).A.String()
		ips[ip] = true
	}

	// 应该看到来自 3 个不同集群的 IP
	assert.Len(t, ips, 3)
	assert.True(t, ips["10.96.1.10"])
	assert.True(t, ips["10.96.2.10"])
	assert.True(t, ips["10.96.3.10"])
}

// TestDNSServer_WithLoadBalancerHandler_NonClustersetDomain 测试非 clusterset 域名
// 应该不会被 LoadBalancer 处理，而是尝试从本地记录中查找
func TestDNSServer_WithLoadBalancerHandler_NonClustersetDomain(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:15372")
	dnsSrv.Start()
	defer dnsSrv.Stop()

	time.Sleep(100 * time.Millisecond)

	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	lb := NewLoadBalancer(sd, dnsSrv, nil)
	dnsSrv.RegisterQueryHandler(lb.HandleQuery)

	// 添加本地 DNS 记录
	dnsSrv.AddRecord("myapp.default.svc.cluster.local.", dns.TypeA, 300, "10.244.0.10")

	c := new(dns.Client)

	// 查询 cluster.local 域名（不是 clusterset.remote）
	m := new(dns.Msg)
	m.SetQuestion("myapp.default.svc.cluster.local.", dns.TypeA)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, dnsSrv.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r.Rcode)
	assert.Len(t, r.Answer, 1)
	assert.Equal(t, "10.244.0.10", r.Answer[0].(*dns.A).A.String())
}

// TestDNSServer_WithLoadBalancerHandler_ClustersetNotFound 测试 clusterset 域名
// 但没有对应服务的情况
func TestDNSServer_WithLoadBalancerHandler_ClustersetNotFound(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:15373")
	dnsSrv.Start()
	defer dnsSrv.Stop()

	time.Sleep(100 * time.Millisecond)

	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	// 不预填充任何服务
	lb := NewLoadBalancer(sd, dnsSrv, nil)
	dnsSrv.RegisterQueryHandler(lb.HandleQuery)

	c := new(dns.Client)

	m := new(dns.Msg)
	m.SetQuestion("nonexistent.default.svc.clusterset.remote.", dns.TypeA)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, dnsSrv.GetAddr())
	assert.NoError(t, err)
	// 应该返回空的 Answer（没有错误，只是没有找到记录）
	assert.Equal(t, dns.RcodeSuccess, r.Rcode)
	assert.Len(t, r.Answer, 0)
}

// TestDNSServer_WithLoadBalancerHandler_ClustersetIPv6 测试 IPv6 查询
func TestDNSServer_WithLoadBalancerHandler_ClustersetIPv6(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:15374")
	dnsSrv.Start()
	defer dnsSrv.Stop()

	time.Sleep(100 * time.Millisecond)

	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "fd7a:115c:a1e0::1"},
				},
			},
			Count: 1,
		},
	}

	lb := NewLoadBalancer(sd, dnsSrv, nil)
	dnsSrv.RegisterQueryHandler(lb.HandleQuery)

	c := new(dns.Client)

	// 查询 AAAA 记录
	m := new(dns.Msg)
	m.SetQuestion("myapp.default.svc.clusterset.remote.", dns.TypeAAAA)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, dnsSrv.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r.Rcode)
	assert.Len(t, r.Answer, 1)
	assert.Equal(t, "fd7a:115c:a1e0::1", r.Answer[0].(*dns.AAAA).AAAA.String())
}

// TestDNSServer_WithLoadBalancerHandler_ClustersetDifferentNamespace 测试不同命名空间
func TestDNSServer_WithLoadBalancerHandler_ClustersetDifferentNamespace(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:15375")
	dnsSrv.Start()
	defer dnsSrv.Stop()

	time.Sleep(100 * time.Millisecond)

	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"api.production": {
					{Name: "api", Namespace: "production", ClusterIP: "10.96.10.10"},
				},
				"db.staging": {
					{Name: "db", Namespace: "staging", ClusterIP: "10.96.20.20"},
				},
			},
			Count: 2,
		},
	}

	lb := NewLoadBalancer(sd, dnsSrv, nil)
	dnsSrv.RegisterQueryHandler(lb.HandleQuery)

	c := new(dns.Client)

	// 测试 api.production
	m1 := new(dns.Msg)
	m1.SetQuestion("api.production.svc.clusterset.remote.", dns.TypeA)
	m1.RecursionDesired = false

	r1, _, err := c.Exchange(m1, dnsSrv.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r1.Rcode)
	assert.Len(t, r1.Answer, 1)
	assert.Equal(t, "10.96.10.10", r1.Answer[0].(*dns.A).A.String())

	// 测试 db.staging
	m2 := new(dns.Msg)
	m2.SetQuestion("db.staging.svc.clusterset.remote.", dns.TypeA)
	m2.RecursionDesired = false

	r2, _, err := c.Exchange(m2, dnsSrv.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r2.Rcode)
	assert.Len(t, r2.Answer, 1)
	assert.Equal(t, "10.96.20.20", r2.Answer[0].(*dns.A).A.String())
}

// TestDNSServer_WithLoadBalancerHandler_ClustersetInvalidDomain 测试无效的 clusterset 域名格式
func TestDNSServer_WithLoadBalancerHandler_ClustersetInvalidDomain(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:15376")
	dnsSrv.Start()
	defer dnsSrv.Stop()

	time.Sleep(100 * time.Millisecond)

	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
			Count: 1,
		},
	}

	lb := NewLoadBalancer(sd, dnsSrv, nil)
	dnsSrv.RegisterQueryHandler(lb.HandleQuery)

	c := new(dns.Client)

	// 无效的域名格式：缺少命名空间部分
	m := new(dns.Msg)
	m.SetQuestion("invalid.clusterset.remote.", dns.TypeA)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, dnsSrv.GetAddr())
	assert.NoError(t, err)
	// 无效域名应该返回空答案
	assert.Equal(t, dns.RcodeSuccess, r.Rcode)
	assert.Len(t, r.Answer, 0)
}

// TestDNSServer_WithLoadBalancerHandler_LocalRecordPriority 测试本地记录的优先级高于 Handler
func TestDNSServer_WithLoadBalancerHandler_LocalRecordPriority(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:15377")
	dnsSrv.Start()
	defer dnsSrv.Stop()

	time.Sleep(100 * time.Millisecond)

	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
			Count: 1,
		},
	}

	lb := NewLoadBalancer(sd, dnsSrv, nil)
	dnsSrv.RegisterQueryHandler(lb.HandleQuery)

	// 添加本地 DNS 记录（与 clusterset 域名相同）
	// 注意：handleDNSRequest 中 Handler 返回 answers 后会 break，
	// 所以不会继续查找本地记录
	dnsSrv.AddRecord("myapp.default.svc.clusterset.remote.", dns.TypeA, 300, "10.244.0.10")

	c := new(dns.Client)

	// 由于 LoadBalancer.HandleQuery 会先被调用并返回结果，
	// 本地记录不应该被使用
	m := new(dns.Msg)
	m.SetQuestion("myapp.default.svc.clusterset.remote.", dns.TypeA)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, dnsSrv.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r.Rcode)
	assert.Len(t, r.Answer, 1)
	// 应该返回负载均衡器提供的 IP，而不是本地记录的 IP
	assert.Equal(t, "10.96.1.10", r.Answer[0].(*dns.A).A.String())
}

// TestDNSServer_WithLoadBalancerHandler_MultipleHandlers 测试多个 Handler 的场景
func TestDNSServer_WithLoadBalancerHandler_MultipleHandlers(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer("127.0.0.1:15378")
	dnsSrv.Start()
	defer dnsSrv.Stop()

	time.Sleep(100 * time.Millisecond)

	mockPeerLister := &MockPeerLister{}
	sd := NewServiceDiscovery(mockPeerLister)
	sd.serviceCache = map[string]*RemoteServiceList{
		"cluster1": {
			Services: map[string][]RemoteService{
				"myapp.default": {
					{Name: "myapp", Namespace: "default", ClusterIP: "10.96.1.10"},
				},
			},
			Count: 1,
		},
	}

	lb := NewLoadBalancer(sd, dnsSrv, nil)

	// 注册一个空的 Handler 作为前置
	dnsSrv.RegisterQueryHandler(func(domain string, qtype uint16) ([]dns.RR, bool) {
		// 不处理任何域名
		return nil, false
	})

	// 注册 LoadBalancer
	dnsSrv.RegisterQueryHandler(lb.HandleQuery)

	c := new(dns.Client)

	m := new(dns.Msg)
	m.SetQuestion("myapp.default.svc.clusterset.remote.", dns.TypeA)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, dnsSrv.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r.Rcode)
	assert.Len(t, r.Answer, 1)
	assert.Equal(t, "10.96.1.10", r.Answer[0].(*dns.A).A.String())
}
