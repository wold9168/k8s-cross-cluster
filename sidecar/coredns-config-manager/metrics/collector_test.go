package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestNewMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector()
	assert.NotNil(t, collector)
	assert.NotNil(t, collector.Base, "should embed base collector")
	assert.NotNil(t, collector.DnsRecordCount)
	assert.NotNil(t, collector.DnsServiceCount)
	assert.NotNil(t, collector.DnsClusterCount)
}

func TestSetRecordCount(t *testing.T) {
	collector := NewMetricsCollector()
	collector.SetRecordCount(10)
	assert.Equal(t, 10, collector.recordCount)
	collector.SetRecordCount(20)
	assert.Equal(t, 20, collector.recordCount)
}

func TestSetServiceClusterCount(t *testing.T) {
	collector := NewMetricsCollector()
	collector.SetServiceCount(5)
	collector.SetClusterCount(3)
	assert.Equal(t, 5, collector.serviceCount)
	assert.Equal(t, 3, collector.clusterCount)
}

func TestCollect(t *testing.T) {
	collector := NewMetricsCollector()
	collector.SetRecordCount(5)
	collector.SetServiceCount(2)
	collector.SetClusterCount(1)

	ch := make(chan prometheus.Metric, 100)
	go func() {
		collector.Collect(ch)
		close(ch)
	}()

	metricCount := 0
	for range ch {
		metricCount++
	}
	assert.GreaterOrEqual(t, metricCount, 3)
}

func TestDescribe(t *testing.T) {
	collector := NewMetricsCollector()

	ch := make(chan *prometheus.Desc, 100)
	go func() {
		collector.Describe(ch)
		close(ch)
	}()

	descCount := 0
	for range ch {
		descCount++
	}
	assert.GreaterOrEqual(t, descCount, 20)
}

func TestGetAllMetrics(t *testing.T) {
	collector := NewMetricsCollector()
	collector.SetRecordCount(10)
	collector.SetServiceCount(5)
	collector.SetClusterCount(3)

	metrics := collector.GetAllMetrics()

	assert.NotNil(t, metrics)
	assert.Contains(t, metrics, "business")
	assert.Contains(t, metrics, "go_runtime")
	assert.Contains(t, metrics, "cpu")
	assert.Contains(t, metrics, "memory")
	assert.Contains(t, metrics, "disk")
	assert.Contains(t, metrics, "network")

	business, ok := metrics["business"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, 10, business["dns_record_count"])
	assert.Equal(t, 5, business["dns_service_count"])
	assert.Equal(t, 3, business["dns_cluster_count"])
}
