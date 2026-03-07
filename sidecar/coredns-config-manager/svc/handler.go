package svc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	dnsserver "github.com/wold9168/k8s-cross-cluster/sidecar/coredns-config-manager/dnsserver"
	k8sclient "github.com/wold9168/k8s-cross-cluster/lib/k8sclient"
)

// Handler 提供 HTTP API 接口
type Handler struct {
	dnsSrv    *dnsserver.DNSServer
	clientset kubernetes.Interface
	nodeName  string
}

// NewHandler 创建新的 HTTP handler
func NewHandler(dnsSrv *dnsserver.DNSServer, clientset kubernetes.Interface, nodeName string) *Handler {
	return &Handler{
		dnsSrv:    dnsSrv,
		clientset: clientset,
		nodeName:  nodeName,
	}
}

// handleAllRecords 处理 /allrecords 端点，返回所有 DNS 记录
func (h *Handler) handleAllRecords(w http.ResponseWriter, r *http.Request) {
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

// handleSvc 处理 /svc 端点，返回当前集群的所有服务（clusterset 格式）
func (h *Handler) handleSvc(w http.ResponseWriter, r *http.Request) {
	// 获取当前命名空间的所有服务
	serviceList, err := k8sclient.GetAllServicesInCurrentNamespace(h.clientset, nil)
	if err != nil {
		klog.Errorf("Failed to get services: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get services: %v", err), http.StatusInternalServerError)
		return
	}

	// 构建服务映射，key 格式为 "serviceName.namespace"
	services := make(map[string][]map[string]interface{})
	for _, svc := range serviceList.Items {
		// 跳过 Headless 服务（没有 ClusterIP）
		if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
			continue
		}

		key := fmt.Sprintf("%s.%s", svc.Name, svc.Namespace)

		// 构建域名：serviceName.namespace.svc.{nodeName}.remote
		clustersetDomain := fmt.Sprintf("%s.%s.svc.%s.remote", svc.Name, svc.Namespace, h.nodeName)

		services[key] = []map[string]interface{}{
			{
				"name":      svc.Name,
				"namespace": svc.Namespace,
				"clusterIP": svc.Spec.ClusterIP,
				"domain":    clustersetDomain,
				"ports":     svc.Spec.Ports,
			},
		}
	}

	// 构建响应
	response := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"services":   services,
		"count":      len(services),
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

// ServeHTTP 实现 http.Handler 接口
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 只处理 GET 请求
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// 根据路径分发到不同的处理函数
	switch r.URL.Path {
	case "/svc":
		h.handleSvc(w, r)
	case "/allrecords":
		h.handleAllRecords(w, r)
	default:
		http.NotFound(w, r)
	}
}

// StartServer 启动 HTTP API 服务器
func StartServer(addr string, dnsSrv *dnsserver.DNSServer, clientset kubernetes.Interface, nodeName string) error {
	handler := NewHandler(dnsSrv, clientset, nodeName)

	mux := http.NewServeMux()
	mux.Handle("/allrecords", handler)
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
	klog.Infof("Allrecords available at http://%s/allrecords", addr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		klog.Errorf("API server error: %v", err)
		return err
	}

	return nil
}
