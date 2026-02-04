package dnsserver

import (
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
)

func TestDNSServer_StartStop(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15353")

	err := server.Start()
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	err = server.Stop()
	assert.NoError(t, err)
}

func TestDNSServer_AddRecord(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15354")
	server.Start()
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	server.AddRecord("test.example.com.", dns.TypeA, 300, "192.168.1.1")

	records := server.GetRecords("test.example.com.")
	assert.Len(t, records, 1)
	assert.Equal(t, "test.example.com.", records[0].Name)
	assert.Equal(t, dns.TypeA, records[0].Type)
	assert.Equal(t, uint32(300), records[0].TTL)
	assert.Equal(t, "192.168.1.1", records[0].Value)
}

func TestDNSServer_UpdateRecords(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15355")
	server.Start()
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	server.AddRecord("update.example.com.", dns.TypeA, 300, "192.168.1.1")

	newRecords := []DNSRecord{
		{Name: "update.example.com.", Type: dns.TypeA, TTL: 600, Value: "192.168.2.1"},
		{Name: "update.example.com.", Type: dns.TypeA, TTL: 600, Value: "192.168.2.2"},
	}
	server.UpdateRecords("update.example.com.", newRecords)

	records := server.GetRecords("update.example.com.")
	assert.Len(t, records, 2)
	assert.Equal(t, "192.168.2.1", records[0].Value)
	assert.Equal(t, "192.168.2.2", records[1].Value)
}

func TestDNSServer_RemoveRecords(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15356")
	server.Start()
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	server.AddRecord("remove.example.com.", dns.TypeA, 300, "192.168.1.1")

	server.RemoveRecords("remove.example.com.")

	records := server.GetRecords("remove.example.com.")
	assert.Nil(t, records)
}

func TestDNSServer_Query(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15357")
	server.Start()
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	server.AddRecord("query.example.com.", dns.TypeA, 300, "192.168.1.1")
	server.AddRecord("query.example.com.", dns.TypeA, 300, "192.168.1.2")

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("query.example.com.", dns.TypeA)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, server.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r.Rcode)
	assert.Len(t, r.Answer, 2)

	assert.Equal(t, "192.168.1.1", r.Answer[0].(*dns.A).A.String())
	assert.Equal(t, "192.168.1.2", r.Answer[1].(*dns.A).A.String())
}

func TestDNSServer_QueryNonExistent(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15358")
	server.Start()
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("nonexistent.example.com.", dns.TypeA)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, server.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r.Rcode)
	assert.Len(t, r.Answer, 0)
}

func TestDNSServer_GetAddr(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15359")
	assert.Equal(t, "127.0.0.1:15359", server.GetAddr())
}

func TestDNSServer_WildcardQuery(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15360")
	server.Start()
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// 添加通配符记录
	server.AddRecord("*.example.com.", dns.TypeA, 300, "10.0.0.1")

	c := new(dns.Client)

	// 测试1-1: 通配符应该匹配子域名
	m1_1 := new(dns.Msg)
	m1_1.SetQuestion("foo.example.com.", dns.TypeA)
	m1_1.RecursionDesired = false

	r1_1, _, err := c.Exchange(m1_1, server.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r1_1.Rcode)
	assert.Len(t, r1_1.Answer, 1)
	assert.Equal(t, "10.0.0.1", r1_1.Answer[0].(*dns.A).A.String())

	// 测试1-2: 通配符应该匹配子域名
	m1_2 := new(dns.Msg)
	m1_2.SetQuestion("bar.example.com.", dns.TypeA)
	m1_2.RecursionDesired = false

	r1_2, _, err := c.Exchange(m1_2, server.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r1_2.Rcode)
	assert.Len(t, r1_2.Answer, 1)
	assert.Equal(t, "10.0.0.1", r1_2.Answer[0].(*dns.A).A.String())

	// 测试2: 通配符不应该匹配父域名
	m2 := new(dns.Msg)
	m2.SetQuestion("example.com.", dns.TypeA)
	m2.RecursionDesired = false

	r2, _, err := c.Exchange(m2, server.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r2.Rcode)
	assert.Len(t, r2.Answer, 0)
}

func TestDNSServer_WildcardExactMatchPriority(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15361")
	server.Start()
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// 添加通配符记录
	server.AddRecord("*.example.com.", dns.TypeA, 300, "10.0.0.1")
	// 添加精确匹配记录
	server.AddRecord("foo.example.com.", dns.TypeA, 300, "10.0.0.2")

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("foo.example.com.", dns.TypeA)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, server.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r.Rcode)
	// 应该返回精确匹配的记录，而不是通配符记录
	assert.Len(t, r.Answer, 1)
	assert.Equal(t, "10.0.0.2", r.Answer[0].(*dns.A).A.String())
}

func TestDNSServer_WildcardMultipleRecords(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15362")
	server.Start()
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// 添加多个通配符记录
	server.AddRecord("*.example.com.", dns.TypeA, 300, "10.0.0.1")
	server.AddRecord("*.example.com.", dns.TypeA, 300, "10.0.0.2")

	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("test.example.com.", dns.TypeA)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, server.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r.Rcode)
	assert.Len(t, r.Answer, 2)
	assert.Equal(t, "10.0.0.1", r.Answer[0].(*dns.A).A.String())
	assert.Equal(t, "10.0.0.2", r.Answer[1].(*dns.A).A.String())
}

func TestDNSServer_WildcardNonExistent(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15363")
	server.Start()
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// 不添加任何通配符记录
	c := new(dns.Client)
	m := new(dns.Msg)
	m.SetQuestion("test.example.com.", dns.TypeA)
	m.RecursionDesired = false

	r, _, err := c.Exchange(m, server.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r.Rcode)
	assert.Len(t, r.Answer, 0)
}

func TestDNSServer_WildcardAddAndGetRecords(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15364")

	// 添加通配符记录
	server.AddRecord("*.example.com.", dns.TypeA, 300, "10.0.0.1")

	// 获取通配符记录
	records := server.GetRecords("*.example.com.")
	assert.Len(t, records, 1)
	assert.Equal(t, "*.example.com.", records[0].Name)
	assert.Equal(t, dns.TypeA, records[0].Type)
	assert.Equal(t, uint32(300), records[0].TTL)
	assert.Equal(t, "10.0.0.1", records[0].Value)
}

func TestDNSServer_WildcardDifferentDomains(t *testing.T) {
	server := NewDNSServer("127.0.0.1:15365")
	server.Start()
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// 添加两个不同域名的通配符记录
	server.AddRecord("*.example.com.", dns.TypeA, 300, "10.0.0.1")
	server.AddRecord("*.test.com.", dns.TypeA, 300, "10.0.0.2")

	c := new(dns.Client)

	// 测试第一个域名的子域名
	m1 := new(dns.Msg)
	m1.SetQuestion("foo.example.com.", dns.TypeA)
	m1.RecursionDesired = false

	r1, _, err := c.Exchange(m1, server.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r1.Rcode)
	assert.Len(t, r1.Answer, 1)
	assert.Equal(t, "10.0.0.1", r1.Answer[0].(*dns.A).A.String())

	// 测试第二个域名的子域名
	m2 := new(dns.Msg)
	m2.SetQuestion("bar.test.com.", dns.TypeA)
	m2.RecursionDesired = false

	r2, _, err := c.Exchange(m2, server.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r2.Rcode)
	assert.Len(t, r2.Answer, 1)
	assert.Equal(t, "10.0.0.2", r2.Answer[0].(*dns.A).A.String())

	// 测试不匹配的域名
	m3 := new(dns.Msg)
	m3.SetQuestion("test.other.com.", dns.TypeA)
	m3.RecursionDesired = false

	r3, _, err := c.Exchange(m3, server.GetAddr())
	assert.NoError(t, err)
	assert.Equal(t, dns.RcodeSuccess, r3.Rcode)
	assert.Len(t, r3.Answer, 0)
}
