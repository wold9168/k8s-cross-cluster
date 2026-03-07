// @title           Caddy Config Manager API
// @version         1.0
// @description     API for managing Caddy configuration and service discovery in Kubernetes clusters
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    https://github.com/wold9168/k8s-cross-cluster
// @contact.email  support@example.com

// @host      localhost:8081
// @BasePath  /

// @securityDefinitions.basic BasicAuth

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// Handler provides HTTP API interface
type Handler struct {
	configManager *ConfigManager
	clientset     kubernetes.Interface
}

// NewHandler creates a new HTTP handler
func NewHandler(cm *ConfigManager, clientset kubernetes.Interface) *Handler {
	return &Handler{
		configManager: cm,
		clientset:      clientset,
	}
}

// @Summary Get current Caddy configuration
// @Description Returns the current generated Caddy configuration
// @Tags config
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Configuration response"
// @Router /config [get]
func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	config, serviceCount, err := h.configManager.GenerateConfigWithCount(ctx)
	if err != nil {
		klog.Errorf("Failed to generate config: %v", err)
		http.Error(w, fmt.Sprintf("Failed to generate config: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"timestamp":    time.Now().Unix(),
		"config":       config,
		"serviceCount": serviceCount,
		"clusterName":  h.configManager.GetClusterName(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		klog.Errorf("Failed to encode config as JSON: %v", err)
	}
}

// @Summary Get all services
// @Description Returns all Kubernetes services in the current namespace
// @Tags services
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Services response"
// @Router /services [get]
func (h *Handler) handleServices(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	serviceList, err := h.configManager.ListServices(ctx)
	if err != nil {
		klog.Errorf("Failed to list services: %v", err)
		http.Error(w, fmt.Sprintf("Failed to list services: %v", err), http.StatusInternalServerError)
		return
	}

	// Filter out headless services
	var services []map[string]interface{}
	for _, svc := range serviceList.Items {
		if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
			continue
		}

		var ports []map[string]interface{}
		for _, port := range svc.Spec.Ports {
			ports = append(ports, map[string]interface{}{
				"name":       port.Name,
				"port":       port.Port,
				"targetPort": port.TargetPort,
				"protocol":   port.Protocol,
			})
		}

		services = append(services, map[string]interface{}{
			"name":      svc.Name,
			"namespace": svc.Namespace,
			"clusterIP": svc.Spec.ClusterIP,
			"type":      svc.Spec.Type,
			"ports":     ports,
		})
	}

	response := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"services":  services,
		"count":     len(services),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		klog.Errorf("Failed to encode services as JSON: %v", err)
	}
}

// @Summary Get domain mappings
// @Description Returns domain mappings between remote and local domains
// @Tags domains
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Domain mapping response"
// @Router /domains [get]
func (h *Handler) handleDomains(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	serviceList, err := h.configManager.ListServices(ctx)
	if err != nil {
		klog.Errorf("Failed to list services: %v", err)
		http.Error(w, fmt.Sprintf("Failed to list services: %v", err), http.StatusInternalServerError)
		return
	}

	domainResult := h.configManager.GenerateDomainMapping(serviceList)

	response := map[string]interface{}{
		"timestamp":     time.Now().Unix(),
		"remoteDomains":  domainResult.RemoteDomains,
		"domainMapping":  domainResult.DomainMapping,
		"totalServices":  len(domainResult.RemoteDomains),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		klog.Errorf("Failed to encode domains as JSON: %v", err)
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

// ServeHTTP implements http.Handler interface
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	switch r.URL.Path {
	case "/config":
		h.handleConfig(w, r)
	case "/services":
		h.handleServices(w, r)
	case "/domains":
		h.handleDomains(w, r)
	default:
		http.NotFound(w, r)
	}
}

// ensure Handler implements http.Handler
var _ http.Handler = (*Handler)(nil)

// StartServer starts the HTTP API server
func StartServer(addr string, cm *ConfigManager, clientset kubernetes.Interface) error {
	handler := NewHandler(cm, clientset)

	mux := http.NewServeMux()
	mux.Handle("/config", handler)
	mux.Handle("/services", handler)
	mux.Handle("/domains", handler)
	mux.HandleFunc("/healthz", healthzHandler)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	klog.Infof("Starting API server on %s", addr)
	klog.Infof("Config available at http://%s/config", addr)
	klog.Infof("Services available at http://%s/services", addr)
	klog.Infof("Domains available at http://%s/domains", addr)
	klog.Infof("Health check available at http://%s/healthz", addr)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		klog.Errorf("API server error: %v", err)
		return err
	}

	return nil
}
