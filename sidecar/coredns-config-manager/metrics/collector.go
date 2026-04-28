package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	metricslib "github.com/wold9168/k8s-cross-cluster/lib/metrics"
)

// MetricsCollector 收集所有指标（系统级 + 业务级）
type MetricsCollector struct {
	Base *metricslib.BaseCollector

	DnsRecordCount  prometheus.Gauge
	DnsServiceCount prometheus.Gauge
	DnsClusterCount prometheus.Gauge

	recordCount  int
	serviceCount int
	clusterCount int

	mu sync.Mutex
}

// NewMetricsCollector 创建新的指标收集器
func NewMetricsCollector() *MetricsCollector {
	c := &MetricsCollector{
		Base: metricslib.NewBaseCollector(),
	}

	c.DnsRecordCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "dns_record_count",
		Help: "Number of DNS records held by the sub-DNS server",
	})
	c.DnsServiceCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "dns_service_count",
		Help: "Number of services discovered across all remote clusters",
	})
	c.DnsClusterCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "dns_cluster_count",
		Help: "Number of remote clusters being serviced",
	})

	return c
}

// Describe 实现 prometheus.Collector 接口
func (c *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.Base.Describe(ch)
	c.DnsRecordCount.Describe(ch)
	c.DnsServiceCount.Describe(ch)
	c.DnsClusterCount.Describe(ch)
}

// Collect 实现 prometheus.Collector 接口
func (c *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	c.Base.Collect(ch)

	c.mu.Lock()
	c.DnsRecordCount.Set(float64(c.recordCount))
	c.DnsServiceCount.Set(float64(c.serviceCount))
	c.DnsClusterCount.Set(float64(c.clusterCount))
	c.mu.Unlock()

	c.DnsRecordCount.Collect(ch)
	c.DnsServiceCount.Collect(ch)
	c.DnsClusterCount.Collect(ch)
}

// GetAllMetrics 返回 JSON 格式的所有指标
func (c *MetricsCollector) GetAllMetrics() map[string]interface{} {
	base := c.Base.GetAllMetrics()

	c.mu.Lock()
	base["business"] = map[string]interface{}{
		"dns_record_count":  c.recordCount,
		"dns_service_count": c.serviceCount,
		"dns_cluster_count": c.clusterCount,
	}
	c.mu.Unlock()

	return base
}

// SetRecordCount 设置 DNS 记录数
func (c *MetricsCollector) SetRecordCount(count int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordCount = count
}

// SetServiceCount 设置服务数量
func (c *MetricsCollector) SetServiceCount(count int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.serviceCount = count
}

// SetClusterCount 设置集群数量
func (c *MetricsCollector) SetClusterCount(count int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clusterCount = count
}
