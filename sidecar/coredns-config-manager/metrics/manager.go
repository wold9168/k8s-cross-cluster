package metrics

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	metricslib "github.com/wold9168/k8s-cross-cluster/lib/metrics"
	"k8s.io/klog/v2"
)

// Manager 管理 metrics 的全局实例
type Manager struct {
	collector *MetricsCollector
	mu        sync.RWMutex
}

var (
	instance *Manager
	once     sync.Once
)

// GetManager 获取全局的 metrics 管理器实例（单例模式）
func GetManager() *Manager {
	once.Do(func() {
		instance = &Manager{
			collector: NewMetricsCollector(),
		}
		klog.Info("Metrics manager initialized")
	})
	return instance
}

// Init 初始化 metrics 管理器
func Init() *Manager {
	return GetManager()
}

// GetCollector 获取指标收集器
func (m *Manager) GetCollector() prometheus.Collector {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.collector
}

// UpdateDNSRecordCount 更新 DNS 记录数
func (m *Manager) UpdateDNSRecordCount(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collector.SetRecordCount(count)
}

// UpdateServiceCount 更新服务数量
func (m *Manager) UpdateServiceCount(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collector.SetServiceCount(count)
}

// UpdateClusterCount 更新集群数量
func (m *Manager) UpdateClusterCount(count int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collector.SetClusterCount(count)
}

// Start 启动 metrics HTTP 服务器
func (m *Manager) Start(addr string) error {
	if m.collector == nil {
		return fmt.Errorf("collector not initialized")
	}
	return metricslib.StartServer(addr, m.collector)
}
