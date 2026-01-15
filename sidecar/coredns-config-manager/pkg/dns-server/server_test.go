package dnsserver

import (
	"fmt"
	"testing"
	"time"

	dns "codeberg.org/miekg/dns"
	"k8s.io/klog/v2"
)

func TestNewServer(t *testing.T) {
	server := NewServer("127.0.0.1:5353")
	if server == nil {
		t.Fatal("NewServer returned nil")
	}
}

func TestServerStartStop(t *testing.T) {
	server := NewServer("127.0.0.1:0") // 使用0端口让系统分配
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	server.Stop()

	// 等待服务器停止
	time.Sleep(100 * time.Millisecond)
}

func TestAddAndGetRecords(t *testing.T) {
	server := NewServer("127.0.0.1:0")

	// 启动服务器以激活更新处理器
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 创建测试记录
	aRecord := CreateARecord("test.example.com.", "192.168.1.1", 300)
	if aRecord == nil {
		t.Fatal("CreateARecord returned nil")
	}

	cnameRecord := CreateCNAMERecord("www.example.com.", "example.com.", 300)
	if cnameRecord == nil {
		t.Fatal("CreateCNAMERecord returned nil")
	}

	// 添加记录
	server.AddRecords("test.example.com.", []dns.RR{aRecord})
	server.AddRecords("www.example.com.", []dns.RR{cnameRecord})

	// 等待异步更新完成
	time.Sleep(50 * time.Millisecond)

	// 获取记录
	records := server.GetRecords("test.example.com.")
	klog.Info(fmt.Sprintln(records))
	if len(records) != 1 {
		t.Errorf("Expected 1 record for test.example.com., got %d", len(records))
	}

	records = server.GetRecords("www.example.com.")
	klog.Info(fmt.Sprintln(records))
	if len(records) != 1 {
		t.Errorf("Expected 1 record for www.example.com., got %d", len(records))
	}

	// 获取不存在的记录
	records = server.GetRecords("nonexistent.example.com.")
	klog.Info(fmt.Sprintln(records))
	if records != nil {
		t.Errorf("Expected nil for nonexistent domain, got %v", records)
	}
}

func TestReplaceRecords(t *testing.T) {
	server := NewServer("127.0.0.1:0")

	// 启动服务器以激活更新处理器
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 添加初始记录
	aRecord1 := CreateARecord("replace.example.com.", "192.168.1.1", 300)
	server.AddRecords("replace.example.com.", []dns.RR{aRecord1})

	// 等待异步更新完成
	time.Sleep(50 * time.Millisecond)

	// 验证初始记录
	records := server.GetRecords("replace.example.com.")
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	// 替换记录
	aRecord2 := CreateARecord("replace.example.com.", "192.168.1.2", 300)
	server.ReplaceRecords("replace.example.com.", []dns.RR{aRecord2})

	// 等待异步更新完成
	time.Sleep(50 * time.Millisecond)

	// 验证替换后的记录
	records = server.GetRecords("replace.example.com.")
	if len(records) != 1 {
		t.Fatalf("Expected 1 record after replace, got %d", len(records))
	}
}

func TestRemoveRecords(t *testing.T) {
	server := NewServer("127.0.0.1:0")

	// 启动服务器以激活更新处理器
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 添加记录
	aRecord := CreateARecord("remove.example.com.", "192.168.1.1", 300)
	server.AddRecords("remove.example.com.", []dns.RR{aRecord})

	// 等待异步更新完成
	time.Sleep(50 * time.Millisecond)

	// 验证记录存在
	records := server.GetRecords("remove.example.com.")
	if len(records) != 1 {
		t.Fatalf("Expected 1 record before remove, got %d", len(records))
	}

	// 移除记录
	server.RemoveRecords("remove.example.com.")

	// 等待异步更新完成
	time.Sleep(50 * time.Millisecond)

	// 验证记录被移除
	records = server.GetRecords("remove.example.com.")
	if records != nil {
		t.Errorf("Expected nil after remove, got %v", records)
	}
}

func TestClearAllRecords(t *testing.T) {
	server := NewServer("127.0.0.1:0")

	// 启动服务器以激活更新处理器
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 添加多个记录
	aRecord1 := CreateARecord("clear1.example.com.", "192.168.1.1", 300)
	aRecord2 := CreateARecord("clear2.example.com.", "192.168.1.2", 300)

	server.AddRecords("clear1.example.com.", []dns.RR{aRecord1})
	server.AddRecords("clear2.example.com.", []dns.RR{aRecord2})

	// 等待异步更新完成
	time.Sleep(50 * time.Millisecond)

	// 验证记录存在
	allRecords := server.GetAllRecords()
	if len(allRecords) != 2 {
		t.Fatalf("Expected 2 records before clear, got %d", len(allRecords))
	}

	// 清除所有记录
	server.ClearAllRecords()

	// 等待异步更新完成
	time.Sleep(50 * time.Millisecond)

	// 验证所有记录被清除
	allRecords = server.GetAllRecords()
	if len(allRecords) != 0 {
		t.Errorf("Expected 0 records after clear, got %d", len(allRecords))
	}
}

func TestGetAllRecords(t *testing.T) {
	server := NewServer("127.0.0.1:0")

	// 启动服务器以激活更新处理器
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 添加多个记录
	aRecord1 := CreateARecord("all1.example.com.", "192.168.1.1", 300)
	aRecord2 := CreateARecord("all2.example.com.", "192.168.1.2", 300)
	cnameRecord := CreateCNAMERecord("www.example.com.", "example.com.", 300)

	server.AddRecords("all1.example.com.", []dns.RR{aRecord1})
	server.AddRecords("all2.example.com.", []dns.RR{aRecord2})
	server.AddRecords("www.example.com.", []dns.RR{cnameRecord})

	// 等待异步更新完成
	time.Sleep(50 * time.Millisecond)

	// 获取所有记录
	allRecords := server.GetAllRecords()

	// 验证记录数量
	if len(allRecords) != 3 {
		t.Errorf("Expected 3 records, got %d", len(allRecords))
	}

	// 验证特定记录存在
	if _, exists := allRecords["all1.example.com."]; !exists {
		t.Error("Record for all1.example.com. not found")
	}
	if _, exists := allRecords["all2.example.com."]; !exists {
		t.Error("Record for all2.example.com. not found")
	}
	if _, exists := allRecords["www.example.com."]; !exists {
		t.Error("Record for www.example.com. not found")
	}
}

func TestCreateARecord(t *testing.T) {
	record := CreateARecord("a.example.com.", "192.168.1.100", 300)
	if record == nil {
		t.Fatal("CreateARecord returned nil")
	}

	// 验证记录类型
	if dns.RRToType(record) != dns.TypeA {
		t.Errorf("Expected TypeA, got %d", dns.RRToType(record))
	}

	// 验证记录内容
	if a, ok := record.(*dns.A); ok {
		if a.A.String() != "192.168.1.100" {
			t.Errorf("Expected IP 192.168.1.100, got %s", a.A.String())
		}
		if a.Hdr.Name != "a.example.com." {
			t.Errorf("Expected name a.example.com., got %s", a.Hdr.Name)
		}
		if a.Hdr.TTL != 300 {
			t.Errorf("Expected TTL 300, got %d", a.Hdr.TTL)
		}
	} else {
		t.Errorf("Expected *dns.A type, got %T", record)
	}
}

func TestCreateCNAMERecord(t *testing.T) {
	record := CreateCNAMERecord("cname.example.com.", "target.example.com.", 600)
	if record == nil {
		t.Fatal("CreateCNAMERecord returned nil")
	}

	// 验证记录类型
	if dns.RRToType(record) != dns.TypeCNAME {
		t.Errorf("Expected TypeCNAME, got %d", dns.RRToType(record))
	}

	// 验证记录内容
	if cname, ok := record.(*dns.CNAME); ok {
		if cname.Target != "target.example.com." {
			t.Errorf("Expected target target.example.com., got %s", cname.Target)
		}
		if cname.Hdr.Name != "cname.example.com." {
			t.Errorf("Expected name cname.example.com., got %s", cname.Hdr.Name)
		}
		if cname.Hdr.TTL != 600 {
			t.Errorf("Expected TTL 600, got %d", cname.Hdr.TTL)
		}
	} else {
		t.Errorf("Expected *dns.CNAME type, got %T", record)
	}
}

func TestCreateTXTRecord(t *testing.T) {
	record := CreateTXTRecord("txt.example.com.", "test text record", 900)
	if record == nil {
		t.Fatal("CreateTXTRecord returned nil")
	}

	// 验证记录类型
	if dns.RRToType(record) != dns.TypeTXT {
		t.Errorf("Expected TypeTXT, got %d", dns.RRToType(record))
	}

	// 验证记录内容
	if txt, ok := record.(*dns.TXT); ok {
		if len(txt.Txt) != 1 || txt.Txt[0] != "test text record" {
			t.Errorf("Expected text 'test text record', got %v", txt.Txt)
		}
		if txt.Hdr.Name != "txt.example.com." {
			t.Errorf("Expected name txt.example.com., got %s", txt.Hdr.Name)
		}
		if txt.Hdr.TTL != 900 {
			t.Errorf("Expected TTL 900, got %d", txt.Hdr.TTL)
		}
	} else {
		t.Errorf("Expected *dns.TXT type, got %T", record)
	}
}

func TestServerGetIP(t *testing.T) {
	server := NewServer("127.0.0.1:5353")

	ip, err := server.GetIP()
	if err != nil {
		t.Fatalf("GetIP failed: %v", err)
	}

	// 验证IP地址格式
	expectedIP := "127.0.0.1:5353"
	if ip != expectedIP {
		t.Errorf("Expected IP %s, got %s", expectedIP, ip)
	}
}

func TestServerGetIPWithLocalhost(t *testing.T) {
	server := NewServer(":5353") // 监听所有接口

	ip, err := server.GetIP()
	if err != nil {
		t.Fatalf("GetIP failed: %v", err)
	}

	// 应该返回一个有效的IP地址
	if ip == "" {
		t.Error("GetIP returned empty string")
	}
}

func TestConcurrentRecordUpdates(t *testing.T) {
	server := NewServer("127.0.0.1:0")

	// 启动服务器以激活更新处理器
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 启动多个goroutine并发添加记录
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			domain := string(rune('a'+id)) + ".example.com."
			ip := "192.168.1." + string(rune('0'+id))
			record := CreateARecord(domain, ip, 300)
			server.AddRecords(domain, []dns.RR{record})
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 等待异步更新完成
	time.Sleep(100 * time.Millisecond)

	// 验证所有记录都被添加
	allRecords := server.GetAllRecords()
	for _,record := range allRecords{
		klog.Info(fmt.Sprintln(record))
	}
	if len(allRecords) != 10 {
		t.Errorf("Expected 10 records after concurrent updates, got %d", len(allRecords))
	}
}

func TestRecordCloneForWildcard(t *testing.T) {
	// 创建原始记录
	originalRecord := CreateARecord("original.example.com.", "192.168.1.1", 300)
	if originalRecord == nil {
		t.Fatal("Failed to create original record")
	}

	// 创建通配符版本
	wildcardRecord := CreateWildcardRecord("*.example.com.", originalRecord)
	if wildcardRecord == nil {
		t.Fatal("CreateWildcardRecord returned nil")
	}

	// 验证通配符记录
	if wildcardRecord.Header().Name != "*.example.com." {
		t.Errorf("Expected wildcard name *.example.com., got %s", wildcardRecord.Header().Name)
	}

	// 验证原始记录未被修改
	if originalRecord.Header().Name != "original.example.com." {
		t.Errorf("Original record name changed to %s", originalRecord.Header().Name)
	}
}
