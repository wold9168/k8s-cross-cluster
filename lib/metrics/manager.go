package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

// Manager 管理 metrics 的全局实例（单例模式）
type Manager struct {
	collector prometheus.Collector
}

var (
	managerInstance *Manager
	// Init初始化为 nil，由调用方设置
	setupFunc func() (prometheus.Collector, *Manager)
)

// SetSetupFunc 设置初始化函数（在首次 Init 调用时执行一次）
// 各 sidecar 在 init() 中调用此函数注册自定义构造逻辑
func SetSetupFunc(fn func() (prometheus.Collector, *Manager)) {
	setupFunc = fn
}

// Init 初始化 metrics 管理器（最多执行一次）
func Init() *Manager {
	if managerInstance == nil {
		if setupFunc == nil {
			klog.Fatal("metrics.SetSetupFunc has not been called before Init")
		}
		collector, mgr := setupFunc()
		managerInstance = mgr
		managerInstance.collector = collector
		klog.Info("Metrics manager initialized")
	}
	return managerInstance
}

// GetCollector 获取指标收集器
func (m *Manager) GetCollector() prometheus.Collector {
	return m.collector
}

// Start 启动 metrics HTTP 服务器
func (m *Manager) Start(addr string) error {
	return StartServer(addr, m.collector)
}
