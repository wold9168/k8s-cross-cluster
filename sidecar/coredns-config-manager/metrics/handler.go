package metrics

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog/v2"
)

// Handler 提供 HTTP metrics 接口
type Handler struct {
	collector *MetricsCollector
	registry  *prometheus.Registry
}

// NewHandler 创建新的 HTTP handler
func NewHandler(collector *MetricsCollector) *Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	return &Handler{
		collector: collector,
		registry:  registry,
	}
}

// ServeHTTP 实现 http.Handler 接口
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 根据查询参数决定输出格式
	format := r.URL.Query().Get("format")
	
	switch format {
	case "json":
		h.serveJSON(w, r)
	default:
		h.servePrometheus(w, r)
	}
}

// servePrometheus 返回 Prometheus 格式的指标
func (h *Handler) servePrometheus(w http.ResponseWriter, r *http.Request) {
	klog.V(4).Info("Serving metrics in Prometheus format")
	
	// 使用 promhttp.HandlerFor 来生成 Prometheus 格式的输出
	handler := promhttp.HandlerFor(h.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: false,
	})
	
	handler.ServeHTTP(w, r)
}

// serveJSON 返回 JSON 格式的指标
func (h *Handler) serveJSON(w http.ResponseWriter, r *http.Request) {
	klog.V(4).Info("Serving metrics in JSON format")
	
	// 收集所有指标
	metrics := h.collector.GetAllMetrics()
	
	// 添加元数据
	response := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"metrics":   metrics,
	}
	
	// 设置响应头
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	// 编码为 JSON
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		klog.Errorf("Failed to encode metrics as JSON: %v", err)
		http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
		return
	}
}

// StartServer 启动 HTTP metrics 服务器
func StartServer(addr string, collector *MetricsCollector) error {
	handler := NewHandler(collector)
	
	mux := http.NewServeMux()
	mux.Handle("/metrics", handler)
	
	// 添加健康检查端点
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	
	klog.Infof("Starting metrics server on %s", addr)
	klog.Infof("Metrics available at http://%s/metrics (Prometheus format)", addr)
	klog.Infof("Metrics available at http://%s/metrics?format=json (JSON format)", addr)
	
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		klog.Errorf("Metrics server error: %v", err)
		return err
	}
	
	return nil
}