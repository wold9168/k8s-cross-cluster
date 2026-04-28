package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	metricslib "github.com/wold9168/k8s-cross-cluster/lib/metrics"
)

// MetricsCollector 收集所有指标（系统级 + 业务级）
type MetricsCollector struct {
	Base *metricslib.BaseCollector

	ConfigUpdateCount prometheus.Counter
	LastConfigUpdate  prometheus.Gauge
	ServiceCount      prometheus.Gauge

	configUpdateCountValue int
	lastConfigUpdateValue  time.Time
	serviceCountValue      int

	mu sync.Mutex
}

// NewMetricsCollector 创建新的指标收集器
func NewMetricsCollector() *MetricsCollector {
	c := &MetricsCollector{
		Base: metricslib.NewBaseCollector(),
	}

	c.ConfigUpdateCount = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "caddy_config_update_total",
		Help: "Total number of Caddy ConfigMap updates since service start",
	})
	c.LastConfigUpdate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "caddy_last_config_update_timestamp",
		Help: "Timestamp of the last Caddy ConfigMap update (Unix epoch seconds)",
	})
	c.ServiceCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "caddy_service_count",
		Help: "Number of services discovered for Caddy configuration",
	})

	return c
}

// Describe 实现 prometheus.Collector 接口
func (c *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.Base.Describe(ch)
	c.ConfigUpdateCount.Describe(ch)
	c.LastConfigUpdate.Describe(ch)
	c.ServiceCount.Describe(ch)
}

// Collect 实现 prometheus.Collector 接口
func (c *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	c.Base.Collect(ch)

	c.mu.Lock()
	if !c.lastConfigUpdateValue.IsZero() {
		c.LastConfigUpdate.Set(float64(c.lastConfigUpdateValue.Unix()))
	}
	c.ServiceCount.Set(float64(c.serviceCountValue))
	c.mu.Unlock()

	c.ConfigUpdateCount.Collect(ch)
	c.LastConfigUpdate.Collect(ch)
	c.ServiceCount.Collect(ch)
}

// GetAllMetrics 返回 JSON 格式的所有指标
func (c *MetricsCollector) GetAllMetrics() map[string]interface{} {
	base := c.Base.GetAllMetrics()

	c.mu.Lock()
	base["business"] = map[string]interface{}{
		"config_update_total":          c.configUpdateCountValue,
		"last_config_update_timestamp": c.lastConfigUpdateValue.Unix(),
		"service_count":                c.serviceCountValue,
	}
	c.mu.Unlock()

	return base
}

// UpdateConfigUpdate 更新配置更新计数（单调递增）
func (c *MetricsCollector) UpdateConfigUpdate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configUpdateCountValue++
	c.lastConfigUpdateValue = time.Now()
}

// SetServiceCount 设置服务数量
func (c *MetricsCollector) SetServiceCount(count int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serviceCountValue = count
}
