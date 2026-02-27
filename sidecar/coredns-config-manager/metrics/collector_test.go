package metrics

import (
	"runtime"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestNewMetricsCollector(t *testing.T) {
	collector := NewMetricsCollector()
	assert.NotNil(t, collector)
	assert.NotNil(t, collector.dnsRecordCount)
	assert.NotNil(t, collector.goroutines)
	assert.NotNil(t, collector.gcCycles)
	assert.NotNil(t, collector.gcDuration)
	assert.NotNil(t, collector.heapAlloc)
	assert.NotNil(t, collector.heapSys)
	assert.NotNil(t, collector.gcCPUPct)
	assert.NotNil(t, collector.threads)
	assert.NotNil(t, collector.cpuUsage)
	assert.NotNil(t, collector.cpuLoad)
	assert.NotNil(t, collector.ctxSwitch)
	assert.NotNil(t, collector.memUsed)
	assert.NotNil(t, collector.memFree)
	assert.NotNil(t, collector.memPercent)
	assert.NotNil(t, collector.diskUsage)
	assert.NotNil(t, collector.diskIOPS)
	assert.NotNil(t, collector.diskLatency)
	assert.NotNil(t, collector.diskRead)
	assert.NotNil(t, collector.diskWrite)
	assert.NotNil(t, collector.netBandwidthRx)
	assert.NotNil(t, collector.netBandwidthTx)
	assert.NotNil(t, collector.netConnections)
	assert.NotNil(t, collector.netDropRate)
	assert.NotNil(t, collector.tcpRetransmit)
}

func TestSetRecordCount(t *testing.T) {
	collector := NewMetricsCollector()

	// 测试设置记录数
	collector.SetRecordCount(10)
	assert.Equal(t, 10, collector.recordCount)

	// 测试更新记录数
	collector.SetRecordCount(20)
	assert.Equal(t, 20, collector.recordCount)
}

func TestCollect(t *testing.T) {
	collector := NewMetricsCollector()
	collector.SetRecordCount(5)

	// 测试 Collect 方法
	ch := make(chan prometheus.Metric, 100)
	
	// 在 goroutine 中调用 Collect，完成后关闭通道
	go func() {
		collector.Collect(ch)
		close(ch)
	}()
	
	// 验证收集到的指标
	metricCount := 0
	for range ch {
		metricCount++
	}

	// 至少应该收集到 dns_record_count 指标
	assert.GreaterOrEqual(t, metricCount, 1)
}

func TestDescribe(t *testing.T) {
	collector := NewMetricsCollector()

	// 测试 Describe 方法
	ch := make(chan *prometheus.Desc, 100)
	
	// 在 goroutine 中调用 Describe，完成后关闭通道
	go func() {
		collector.Describe(ch)
		close(ch)
	}()
	
	// 验证描述信息
	descCount := 0
	for range ch {
		descCount++
	}

	// 应该有所有的指标描述
	assert.GreaterOrEqual(t, descCount, 20)
}

func TestCollectGoRuntimeMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	// 收集 Go 运行时指标
	collector.collectGoRuntimeMetrics()

	// 验证指标值是否被设置
	var gGoroutines dto.Metric
	err := collector.goroutines.Write(&gGoroutines)
	assert.NoError(t, err)
	assert.Greater(t, gGoroutines.GetGauge().GetValue(), float64(0))

	// 验证 goroutine 数量与运行时值一致
	assert.Equal(t, float64(runtime.NumGoroutine()), gGoroutines.GetGauge().GetValue())
}

func TestCollectMemoryMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	// 收集内存指标
	collector.collectMemoryMetrics()

	// 验证指标值是否被设置
	var gMemUsed dto.Metric
	err := collector.memUsed.Write(&gMemUsed)
	assert.NoError(t, err)
	assert.Greater(t, gMemUsed.GetGauge().GetValue(), float64(0))
}

func TestCollectCPUMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	// 收集 CPU 指标
	collector.collectCPUMetrics()

	// 验证指标值是否被设置（CPU 使用率应该在 0-100 之间）
	var gCPUUsage dto.Metric
	err := collector.cpuUsage.Write(&gCPUUsage)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, gCPUUsage.GetGauge().GetValue(), float64(0))
	assert.LessOrEqual(t, gCPUUsage.GetGauge().GetValue(), float64(100))
}

func TestGetAllMetrics(t *testing.T) {
	collector := NewMetricsCollector()
	collector.SetRecordCount(10)

	// 获取所有指标
	metrics := collector.GetAllMetrics()

	// 验证返回的数据结构
	assert.NotNil(t, metrics)
	assert.Contains(t, metrics, "business")
	assert.Contains(t, metrics, "go_runtime")
	assert.Contains(t, metrics, "cpu")
	assert.Contains(t, metrics, "memory")
	assert.Contains(t, metrics, "disk")
	assert.Contains(t, metrics, "network")

	// 验证业务指标
	business, ok := metrics["business"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, 10, business["dns_record_count"])

	// 验证 Go 运行时指标
	goRuntime, ok := metrics["go_runtime"].(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, goRuntime, "goroutines")
	assert.Contains(t, goRuntime, "heap_alloc_bytes")
	assert.Contains(t, goRuntime, "heap_sys_bytes")

	// 验证 goroutine 数量与运行时值一致
	assert.Equal(t, runtime.NumGoroutine(), goRuntime["goroutines"])
}

func TestGetHeapAlloc(t *testing.T) {
	collector := NewMetricsCollector()
	heapAlloc := collector.getHeapAlloc()
	assert.Greater(t, heapAlloc, uint64(0))
}

func TestGetHeapSys(t *testing.T) {
	collector := NewMetricsCollector()
	heapSys := collector.getHeapSys()
	assert.Greater(t, heapSys, uint64(0))
}

func TestGetGCPUPct(t *testing.T) {
	collector := NewMetricsCollector()
	gcPct := collector.getGCPUPct()
	// GC CPU 百分比应该在 0-100 之间（或者为 0，如果没有 GC 发生）
	assert.GreaterOrEqual(t, gcPct, float64(0))
	assert.LessOrEqual(t, gcPct, float64(100))
}

func TestGetCPUUsage(t *testing.T) {
	collector := NewMetricsCollector()
	cpuUsage := collector.getCPUUsage()
	// CPU 使用率应该在 0-100 之间（或者为 0，如果无法获取）
	assert.GreaterOrEqual(t, cpuUsage, float64(0))
	assert.LessOrEqual(t, cpuUsage, float64(100))
}

func TestGetMemUsed(t *testing.T) {
	collector := NewMetricsCollector()
	memUsed := collector.getMemUsed()
	assert.Greater(t, memUsed, uint64(0))
}

func TestGetMemFree(t *testing.T) {
	collector := NewMetricsCollector()
	memFree := collector.getMemFree()
	assert.Greater(t, memFree, uint64(0))
}

func TestGetMemPercent(t *testing.T) {
	collector := NewMetricsCollector()
	memPercent := collector.getMemPercent()
	// 内存使用率应该在 0-100 之间
	assert.GreaterOrEqual(t, memPercent, float64(0))
	assert.LessOrEqual(t, memPercent, float64(100))
}

func TestGetDiskUsage(t *testing.T) {
	collector := NewMetricsCollector()
	diskUsage := collector.getDiskUsage()
	// 磁盘使用率应该在 0-100 之间（或者为 0，如果无法获取）
	assert.GreaterOrEqual(t, diskUsage, float64(0))
	assert.LessOrEqual(t, diskUsage, float64(100))
}

func TestGetDiskIOPS(t *testing.T) {
	collector := NewMetricsCollector()
	diskIOPS := collector.getDiskIOPS()
	// IOPS 应该大于等于 0
	assert.GreaterOrEqual(t, diskIOPS, float64(0))
}

func TestGetDiskRead(t *testing.T) {
	collector := NewMetricsCollector()
	diskRead := collector.getDiskRead()
	// 磁盘读取吞吐量应该大于等于 0
	assert.GreaterOrEqual(t, diskRead, float64(0))
}

func TestGetDiskWrite(t *testing.T) {
	collector := NewMetricsCollector()
	diskWrite := collector.getDiskWrite()
	// 磁盘写入吞吐量应该大于等于 0
	assert.GreaterOrEqual(t, diskWrite, float64(0))
}

func TestGetNetBandwidthRx(t *testing.T) {
	collector := NewMetricsCollector()
	netRx := collector.getNetBandwidthRx()
	// 网络接收带宽应该大于等于 0
	assert.GreaterOrEqual(t, netRx, float64(0))
}

func TestGetNetBandwidthTx(t *testing.T) {
	collector := NewMetricsCollector()
	netTx := collector.getNetBandwidthTx()
	// 网络发送带宽应该大于等于 0
	assert.GreaterOrEqual(t, netTx, float64(0))
}

func TestGetNetConnections(t *testing.T) {
	collector := NewMetricsCollector()
	netConn := collector.getNetConnections()
	// 网络连接数应该大于等于 0
	assert.GreaterOrEqual(t, netConn, float64(0))
}

func TestGetNetDropRate(t *testing.T) {
	collector := NewMetricsCollector()
	netDrop := collector.getNetDropRate()
	// 网络丢包率应该在 0-100 之间
	assert.GreaterOrEqual(t, netDrop, float64(0))
	assert.LessOrEqual(t, netDrop, float64(100))
}

func TestGetTCPRetransmit(t *testing.T) {
	collector := NewMetricsCollector()
	tcpRetrans := collector.getTCPRetransmit()
	// TCP 重传率应该在 0-100 之间
	assert.GreaterOrEqual(t, tcpRetrans, float64(0))
	assert.LessOrEqual(t, tcpRetrans, float64(100))
}

func TestCollectDiskMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	// 收集磁盘指标
	collector.collectDiskMetrics()

	// 验证指标值是否被设置
	var gDiskUsage dto.Metric
	err := collector.diskUsage.Write(&gDiskUsage)
	assert.NoError(t, err)
	// 磁盘使用率应该在 0-100 之间（或者为 0，如果无法获取）
	assert.GreaterOrEqual(t, gDiskUsage.GetGauge().GetValue(), float64(0))
	assert.LessOrEqual(t, gDiskUsage.GetGauge().GetValue(), float64(100))
}

func TestCollectNetworkMetrics(t *testing.T) {
	collector := NewMetricsCollector()

	// 收集网络指标
	collector.collectNetworkMetrics()

	// 验证指标值是否被设置
	var gNetBandwidthRx dto.Metric
	err := collector.netBandwidthRx.Write(&gNetBandwidthRx)
	assert.NoError(t, err)
	// 网络带宽应该大于等于 0
	assert.GreaterOrEqual(t, gNetBandwidthRx.GetGauge().GetValue(), float64(0))
}