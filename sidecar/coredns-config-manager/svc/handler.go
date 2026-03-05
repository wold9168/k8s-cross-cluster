package svc

import (
	"encoding/json"
	"net/http"
	"time"

	"k8s.io/klog/v2"

	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
)

// Handler 提供 /svc API 接口
type Handler struct {
	dnsSrv *dnsserver.DNSServer
}

// NewHandler 创建新的 HTTP handler
func NewHandler(dnsSrv *dnsserver.DNSServer) *Handler {
	return &Handler{
		dnsSrv: dnsSrv,
	}
}

// ServeHTTP 实现 http.Handler 接口
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 只处理 GET 请求
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// 获取所有 DNS 记录
	records := h.dnsSrv.GetAllRecords()

	// 构建响应
	response := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"services":  records,
		"count":     len(records),
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// 编码为 JSON
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		klog.Errorf("Failed to encode services as JSON: %v", err)
		http.Error(w, "Failed to encode services", http.StatusInternalServerError)
		return
	}
}

// StartServer 启动 HTTP API 服务器
func StartServer(addr string, dnsSrv *dnsserver.DNSServer) error {
	handler := NewHandler(dnsSrv)

	mux := http.NewServeMux()
	mux.Handle("/svc", handler)

	// 添加健康检查端点
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	klog.Infof("Starting API server on %s", addr)
	klog.Infof("Services available at http://%s/svc", addr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		klog.Errorf("API server error: %v", err)
		return err
	}

	return nil
}