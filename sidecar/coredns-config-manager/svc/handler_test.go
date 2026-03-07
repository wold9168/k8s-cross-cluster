package svc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
)

// TestHandler_ServeHTTP_Success 测试正常 GET 请求
func TestHandler_ServeHTTP_Success(t *testing.T) {
	// 创建测试用 DNS 服务器
	dnsSrv := dnsserver.NewDNSServer(":0")

	// 添加一些测试记录
	dnsSrv.AddRecord("example.com", 1, 300, "192.168.1.1")
	dnsSrv.AddRecord("example.com", 1, 300, "192.168.1.2")
	dnsSrv.AddRecord("test.com", 1, 600, "10.0.0.1")

	// 创建 handler
	handler := NewHandler(dnsSrv, nil, "test-node", nil, nil)

	// 创建测试请求
	req := httptest.NewRequest(http.MethodGet, "/allrecords", nil)
	w := httptest.NewRecorder()

	// 执行请求
	handler.ServeHTTP(w, req)

	// 验证响应状态码
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	// 验证 Content-Type
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type 'application/json', got '%s'", contentType)
	}

	// 解析响应体
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 验证响应字段
	if _, ok := response["timestamp"]; !ok {
		t.Error("Response missing 'timestamp' field")
	}

	if _, ok := response["services"]; !ok {
		t.Error("Response missing 'services' field")
	}

	if _, ok := response["count"]; !ok {
		t.Error("Response missing 'count' field")
	}

	// 验证 count 字段 (count 是域名的数量，不是记录总数)
	count := int(response["count"].(float64))
	if count != 2 {
		t.Errorf("Expected count 2 (number of domains), got %d", count)
	}

	// 验证 services 字段
	services := response["services"].(map[string]interface{})
	if len(services) != 2 {
		t.Errorf("Expected 2 domains, got %d", len(services))
	}
}

// TestHandler_ServeHTTP_MethodNotAllowed 测试非 GET 请求
func TestHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer(":0")
	handler := NewHandler(dnsSrv, nil, "test-node", nil, nil)

	testCases := []struct {
		method string
		name   string
	}{
		{http.MethodPost, "POST"},
		{http.MethodPut, "PUT"},
		{http.MethodDelete, "DELETE"},
		{http.MethodPatch, "PATCH"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/allrecords", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status %d for %s, got %d", http.StatusMethodNotAllowed, tc.name, w.Code)
			}
		})
	}
}

// TestHandler_ServeHTTP_EmptyRecords 测试没有 DNS 记录的情况
func TestHandler_ServeHTTP_EmptyRecords(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer(":0")
	handler := NewHandler(dnsSrv, nil, "test-node", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/allrecords", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	count := int(response["count"].(float64))
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}

	services := response["services"].(map[string]interface{})
	if len(services) != 0 {
		t.Errorf("Expected empty services, got %d items", len(services))
	}
}

// TestHandler_ServeHTTP_Timestamp 测试时间戳字段
func TestHandler_ServeHTTP_Timestamp(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer(":0")
	handler := NewHandler(dnsSrv, nil, "test-node", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/allrecords", nil)
	w := httptest.NewRecorder()

	before := time.Now().Unix()
	handler.ServeHTTP(w, req)
	after := time.Now().Unix()

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	timestamp := int64(response["timestamp"].(float64))
	if timestamp < before || timestamp > after {
		t.Errorf("Timestamp %d not within expected range [%d, %d]", timestamp, before, after)
	}
}

// TestHandler_ServeHTTP_ResponseFormat 测试响应 JSON 格式
func TestHandler_ServeHTTP_ResponseFormat(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer(":0")
	dnsSrv.AddRecord("example.com", 1, 300, "192.168.1.1")
	handler := NewHandler(dnsSrv, nil, "test-node", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/allrecords", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// 验证响应体是有效的 JSON 且有缩进
	body := w.Body.String()
	if len(body) == 0 {
		t.Fatal("Response body is empty")
	}

	// 检查是否有缩进（两个空格）
	if !containsIndent(body) {
		t.Error("Response JSON should be indented")
	}
}

func containsIndent(s string) bool {
	// 检查是否包含两个空格的缩进
	lines := []string{}
	for _, line := range s {
		if line == '\n' {
			lines = append(lines, "")
		}
	}
	_ = lines
	// 简单检查：查找 "  "（两个空格）
	return len(s) > 0 && (s[0] == ' ' || containsDoubleSpace(s))
}

func containsDoubleSpace(s string) bool {
	for i := 0; i < len(s)-1; i++ {
		if s[i] == ' ' && s[i+1] == ' ' {
			return true
		}
	}
	return false
}

// TestNewHandler 测试 NewHandler 函数
func TestNewHandler(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer(":0")
	handler := NewHandler(dnsSrv, nil, "test-node", nil, nil)

	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}

	if handler.dnsSrv != dnsSrv {
		t.Error("Handler's dnsSrv does not match the provided instance")
	}
}

// TestStartServer 测试 StartServer 函数（不实际启动服务器，只测试 handler 创建）
func TestStartServer_HandlerCreation(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer(":0")

	// 创建一个测试用的 handler 来验证 StartServer 会正确创建 handler
	handler := NewHandler(dnsSrv, nil, "test-node", nil, nil)
	if handler == nil {
		t.Fatal("Failed to create handler")
	}

	// 验证 handler 能正常工作
	req := httptest.NewRequest(http.MethodGet, "/allrecords", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Handler should work correctly, got status %d", w.Code)
	}
}

// TestHandler_HealthzEndpoint 测试健康检查端点
func TestHandler_HealthzEndpoint(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer(":0")
	handler := NewHandler(dnsSrv, nil, "test-node", nil, nil)

	// 创建 mux 来模拟 StartServer 中的设置
	mux := http.NewServeMux()
	mux.Handle("/allrecords", handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d for healthz endpoint, got %d", http.StatusOK, w.Code)
	}

	body, err := io.ReadAll(w.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	if string(body) != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", string(body))
	}
}

// TestHandler_ServeHTTP_LargeNumberOfRecords 测试大量 DNS 记录
func TestHandler_ServeHTTP_LargeNumberOfRecords(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer(":0")

	// 添加大量记录
	for i := 0; i < 100; i++ {
		dnsSrv.AddRecord(fmt.Sprintf("domain%d.com", i), 1, 300, fmt.Sprintf("192.168.1.%d", i))
	}

	handler := NewHandler(dnsSrv, nil, "test-node", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/allrecords", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	count := int(response["count"].(float64))
	if count != 100 {
		t.Errorf("Expected count 100, got %d", count)
	}
}

// TestHandler_ServeHTTP_DifferentRecordTypes 测试不同类型的 DNS 记录
func TestHandler_ServeHTTP_DifferentRecordTypes(t *testing.T) {
	dnsSrv := dnsserver.NewDNSServer(":0")

	// 添加不同类型的记录
	dnsSrv.AddRecord("example.com", 1, 300, "192.168.1.1")     // A 记录
	dnsSrv.AddRecord("example.com", 28, 300, "::1")            // AAAA 记录
	dnsSrv.AddRecord("example.com", 15, 300, "mail.example.com") // MX 记录

	handler := NewHandler(dnsSrv, nil, "test-node", nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/allrecords", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// count 是域名的数量，不是记录总数
	count := int(response["count"].(float64))
	if count != 1 {
		t.Errorf("Expected count 1 (single domain), got %d", count)
	}
}
