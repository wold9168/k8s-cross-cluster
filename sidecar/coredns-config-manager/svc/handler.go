// @title           CoreDNS Config Manager API
// @version         1.0
// @description     API for managing DNS configuration and service discovery in Kubernetes clusters
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    https://github.com/wold9168/k8s-cross-cluster
// @contact.email  support@example.com

// @host      localhost:8081
// @BasePath  /

// @securityDefinitions.basic BasicAuth

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

// LBStatus represents the status of a single service key in the load balancer
type LBStatus struct {
	ServiceKey  string      `json:"serviceKey"`
	Endpoints   interface{} `json:"endpoints"`
	NextIdx     uint64      `json:"nextIdx"`
	EndpointNum int         `json:"endpointNum"`
}

// LoadBalancerStatusProvider 接口用于获取负载均衡器状态
type LoadBalancerStatusProvider interface {
	GetLoadBalancerStatus() []LBStatus
}

// LoadBalancerRotator 接口用于触发负载均衡器轮转
type LoadBalancerRotator interface {
	RotateAllServices() map[string]uint64
}

// Handler 提供 HTTP API 接口
type Handler struct {
	dnsSrv        *dnsserver.DNSServer
	clientset     kubernetes.Interface
	nodeName      string
	lbStatus      LoadBalancerStatusProvider
	lbRotator     LoadBalancerRotator
}

// NewHandler 创建新的 HTTP handler
func NewHandler(dnsSrv *dnsserver.DNSServer, clientset kubernetes.Interface, nodeName string, lbStatus LoadBalancerStatusProvider, lbRotator LoadBalancerRotator) *Handler {
	return &Handler{
		dnsSrv:        dnsSrv,
		clientset:     clientset,
		nodeName:      nodeName,
		lbStatus:      lbStatus,
		lbRotator:     lbRotator,
	}
}

// @Summary Get all DNS records
// @Description Returns all DNS records managed by the CoreDNS config manager
// @Tags dns
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "DNS records response"
// @Router /allrecords [get]
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

// @Summary Get load balancer status
// @Description Returns the current status of the load balancer, including service keys, endpoints, and the next index value
// @Tags loadbalancer
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Load balancer status response"
// @Router /lbstatus [get]
func (h *Handler) handleLBStatus(w http.ResponseWriter, r *http.Request) {
	if h.lbStatus == nil {
		http.Error(w, "Load balancer not available", http.StatusServiceUnavailable)
		return
	}

	// 获取负载均衡器状态
	status := h.lbStatus.GetLoadBalancerStatus()

	// 构建响应
	response := map[string]interface{}{
		"timestamp":     time.Now().Unix(),
		"serviceKeys":   status,
		"totalServices": len(status),
	}

	// 设置响应头
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// 编码为 JSON
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		klog.Errorf("Failed to encode lbstatus as JSON: %v", err)
		http.Error(w, "Failed to encode lbstatus", http.StatusInternalServerError)
		return
	}
}

// @Summary Get all services in clusterset format
// @Description Returns all Kubernetes services in the current namespace in clusterset format with DNS domain mappings
// @Tags services
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Services response"
// @Router /svc [get]
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
	// 根据路径分发到不同的处理函数
	switch r.URL.Path {
	case "/svc":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		h.handleSvc(w, r)
	case "/allrecords":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		h.handleAllRecords(w, r)
	case "/lbstatus":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		h.handleLBStatus(w, r)
	case "/rotate":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		h.handleRotate(w, r)
	default:
		http.NotFound(w, r)
	}
}

// @Summary Health check
// @Description Returns OK if the service is running
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {string} string "OK"
// @Router /healthz [get]
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// @Summary Trigger load balancer rotation
// @Description Manually triggers round-robin rotation for all discovered services
// @Tags loadbalancer
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Rotation results"
// @Router /rotate [post]
func (h *Handler) handleRotate(w http.ResponseWriter, r *http.Request) {
	if h.lbRotator == nil {
		http.Error(w, "Load balancer rotator not available", http.StatusServiceUnavailable)
		return
	}

	// Trigger rotation for all services
	results := h.lbRotator.RotateAllServices()

	// Build response
	response := map[string]interface{}{
		"timestamp":    time.Now().Unix(),
		"rotated":      len(results),
		"serviceKeys":  results,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Encode as JSON
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		klog.Errorf("Failed to encode rotate response as JSON: %v", err)
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// StartServer 启动 HTTP API 服务器
func StartServer(addr string, dnsSrv *dnsserver.DNSServer, clientset kubernetes.Interface, nodeName string, lbStatus LoadBalancerStatusProvider, lbRotator LoadBalancerRotator) error {
	handler := NewHandler(dnsSrv, clientset, nodeName, lbStatus, lbRotator)

	mux := http.NewServeMux()
	mux.Handle("/allrecords", handler)
	mux.Handle("/svc", handler)
	mux.Handle("/lbstatus", handler)
	mux.Handle("/rotate", handler)

	// 添加健康检查端点
	mux.HandleFunc("/healthz", healthzHandler)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	klog.Infof("Starting API server on %s", addr)
	klog.Infof("Services available at http://%s/svc", addr)
	klog.Infof("Allrecords available at http://%s/allrecords", addr)
	klog.Infof("LBStatus available at http://%s/lbstatus", addr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		klog.Errorf("API server error: %v", err)
		return err
	}

	return nil
}
