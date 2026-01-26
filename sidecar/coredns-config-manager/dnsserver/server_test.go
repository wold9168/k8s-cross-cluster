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
