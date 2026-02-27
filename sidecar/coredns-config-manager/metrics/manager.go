package metrics

import (
	"sync"

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
func (m *Manager) GetCollector() *MetricsCollector {
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

// Start 启动 metrics HTTP 服务器
func (m *Manager) Start(addr string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return StartServer(addr, m.collector)
}
