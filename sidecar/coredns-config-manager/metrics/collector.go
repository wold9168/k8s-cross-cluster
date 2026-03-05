package metrics

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"k8s.io/klog/v2"
)

// MetricsCollector 收集所有应用指标
type MetricsCollector struct {
	// 业务指标
	dnsRecordCount prometheus.Gauge
	recordCount    int // 实际记录数，需要外部设置

	// Go 运行时指标
	goroutines prometheus.Gauge
	gcCycles   prometheus.Gauge
	gcDuration prometheus.Summary
	heapAlloc  prometheus.Gauge
	heapSys    prometheus.Gauge
	gcCPUPct   prometheus.Gauge
	threads    prometheus.Gauge

	// CPU 指标
	cpuUsage   prometheus.Gauge
	cpuLoad    prometheus.Gauge
	ctxSwitch  prometheus.Gauge

	// 内存指标
	memUsed    prometheus.Gauge
	memFree    prometheus.Gauge
	memPercent prometheus.Gauge

	// 磁盘指标
	diskUsage    prometheus.Gauge
	diskIOPS     prometheus.Gauge
	diskLatency  prometheus.Summary
	diskRead     prometheus.Gauge
	diskWrite    prometheus.Gauge

	// 网络指标
	netBandwidthRx prometheus.Gauge
	netBandwidthTx prometheus.Gauge
	netConnections prometheus.Gauge
	netDropRate    prometheus.Gauge
	tcpRetransmit  prometheus.Gauge

	// 用于计算变化率的缓存
	prevNetStats     map[string]net.IOCountersStat
	prevDiskStats    map[string]disk.IOCountersStat
	prevCPUTimes     []cpu.TimesStat
	prevTime         time.Time
	mu               sync.Mutex

	// 用于存储当前值，供 JSON 输出使用
	currentDiskRead    float64
	currentDiskWrite   float64
	currentNetRx       float64
	currentNetTx       float64
	currentNetConn     float64
	currentNetDrop     float64
	currentTCPRetrans  float64
}

// NewMetricsCollector 创建新的指标收集器
func NewMetricsCollector() *MetricsCollector {
	c := &MetricsCollector{
		prevNetStats:  make(map[string]net.IOCountersStat),
		prevDiskStats: make(map[string]disk.IOCountersStat),
		prevTime:      time.Now(),
	}

	// 初始化 Prometheus 指标
	c.initPrometheusMetrics()

	return c
}

// initPrometheusMetrics 初始化 Prometheus 指标
func (c *MetricsCollector) initPrometheusMetrics() {
	// 业务指标
	c.dnsRecordCount = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "dns_record_count",
		Help: "Number of DNS records held by the sub-DNS server",
	})

	// Go 运行时指标
	c.goroutines = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_goroutines",
		Help: "Number of goroutines that currently exist",
	})
	c.gcCycles = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_gc_cycles",
		Help: "Number of GC cycles completed",
	})
	c.gcDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "go_gc_duration_seconds",
		Help: "Time spent in GC since program start in quantiles",
	})
	c.heapAlloc = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_heap_alloc_bytes",
		Help: "Bytes of allocated heap objects",
	})
	c.heapSys = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_heap_sys_bytes",
		Help: "Bytes of heap memory obtained from the OS",
	})
	c.gcCPUPct = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_gc_cpu_percent",
		Help: "Percentage of CPU time used in garbage collection",
	})
	c.threads = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_threads",
		Help: "Number of OS threads created",
	})

	// CPU 指标
	c.cpuUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_usage_percent",
		Help: "CPU usage percentage",
	})
	c.cpuLoad = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_load",
		Help: "System load average",
	})
	c.ctxSwitch = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_context_switches",
		Help: "Number of context switches",
	})

	// 内存指标
	c.memUsed = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "memory_used_bytes",
		Help: "Memory used in bytes",
	})
	c.memFree = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "memory_free_bytes",
		Help: "Memory free in bytes",
	})
	c.memPercent = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "memory_percent",
		Help: "Memory usage percentage",
	})

	// 磁盘指标
	c.diskUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "disk_usage_percent",
		Help: "Disk usage percentage",
	})
	c.diskIOPS = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "disk_iops",
		Help: "Disk I/O operations per second",
	})
	c.diskLatency = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "disk_latency_seconds",
		Help: "Disk read/write latency in quantiles",
	})
	c.diskRead = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "disk_read_bytes_per_sec",
		Help: "Disk read throughput in bytes per second",
	})
	c.diskWrite = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "disk_write_bytes_per_sec",
		Help: "Disk write throughput in bytes per second",
	})

	// 网络指标
	c.netBandwidthRx = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "network_bandwidth_rx_bytes_per_sec",
		Help: "Network receive bandwidth in bytes per second",
	})
	c.netBandwidthTx = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "network_bandwidth_tx_bytes_per_sec",
		Help: "Network transmit bandwidth in bytes per second",
	})
	c.netConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "network_connections",
		Help: "Number of network connections",
	})
	c.netDropRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "network_drop_rate",
		Help: "Network packet drop rate",
	})
	c.tcpRetransmit = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tcp_retransmit_rate",
		Help: "TCP segment retransmission rate",
	})
}

// Describe implements prometheus.Collector
func (c *MetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	c.dnsRecordCount.Describe(ch)
	c.goroutines.Describe(ch)
	c.gcCycles.Describe(ch)
	c.gcDuration.Describe(ch)
	c.heapAlloc.Describe(ch)
	c.heapSys.Describe(ch)
	c.gcCPUPct.Describe(ch)
	c.threads.Describe(ch)
	c.cpuUsage.Describe(ch)
	c.cpuLoad.Describe(ch)
	c.ctxSwitch.Describe(ch)
	c.memUsed.Describe(ch)
	c.memFree.Describe(ch)
	c.memPercent.Describe(ch)
	c.diskUsage.Describe(ch)
	c.diskIOPS.Describe(ch)
	c.diskLatency.Describe(ch)
	c.diskRead.Describe(ch)
	c.diskWrite.Describe(ch)
	c.netBandwidthRx.Describe(ch)
	c.netBandwidthTx.Describe(ch)
	c.netConnections.Describe(ch)
	c.netDropRate.Describe(ch)
	c.tcpRetransmit.Describe(ch)
}

// Collect implements prometheus.Collector
func (c *MetricsCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 收集所有指标
	c.collectGoRuntimeMetrics()
	c.collectCPUMetrics()
	c.collectMemoryMetrics()
	c.collectDiskMetrics()
	c.collectNetworkMetrics()
	c.recordCountMetrics()

	// 发送指标到 Prometheus
	c.dnsRecordCount.Collect(ch)
	c.goroutines.Collect(ch)
	c.gcCycles.Collect(ch)
	c.gcDuration.Collect(ch)
	c.heapAlloc.Collect(ch)
	c.heapSys.Collect(ch)
	c.gcCPUPct.Collect(ch)
	c.threads.Collect(ch)
	c.cpuUsage.Collect(ch)
	c.cpuLoad.Collect(ch)
	c.ctxSwitch.Collect(ch)
	c.memUsed.Collect(ch)
	c.memFree.Collect(ch)
	c.memPercent.Collect(ch)
	c.diskUsage.Collect(ch)
	c.diskIOPS.Collect(ch)
	c.diskLatency.Collect(ch)
	c.diskRead.Collect(ch)
	c.diskWrite.Collect(ch)
	c.netBandwidthRx.Collect(ch)
	c.netBandwidthTx.Collect(ch)
	c.netConnections.Collect(ch)
	c.netDropRate.Collect(ch)
	c.tcpRetransmit.Collect(ch)
}

// SetRecordCount 设置 DNS 记录数
func (c *MetricsCollector) SetRecordCount(count int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recordCount = count
}

// recordCountMetrics 记录业务指标
func (c *MetricsCollector) recordCountMetrics() {
	c.dnsRecordCount.Set(float64(c.recordCount))
}

// collectGoRuntimeMetrics 收集 Go 运行时指标
func (c *MetricsCollector) collectGoRuntimeMetrics() {
	// Goroutine 数量
	c.goroutines.Set(float64(runtime.NumGoroutine()))

	// GC 相关指标
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	c.heapAlloc.Set(float64(memStats.HeapAlloc))
	c.heapSys.Set(float64(memStats.HeapSys))

	// 从 GCStats 获取 GC 信息
	var gcStats debug.GCStats
	gcStats.Pause = make([]time.Duration, 100) // 保留最近 100 次 GC 的耗时
	debug.ReadGCStats(&gcStats)

	if len(gcStats.Pause) > 0 {
		// 手动计算分位数
		sort.Slice(gcStats.Pause, func(i, j int) bool {
			return gcStats.Pause[i] < gcStats.Pause[j]
		})
		// 计算分位数
		pauseCount := len(gcStats.Pause)
		c.gcDuration.Observe(float64(gcStats.Pause[pauseCount*50/100].Seconds()))
		c.gcDuration.Observe(float64(gcStats.Pause[pauseCount*90/100].Seconds()))
		c.gcDuration.Observe(float64(gcStats.Pause[pauseCount*99/100].Seconds()))
	}

	c.gcCycles.Set(float64(gcStats.NumGC))

	// GC 占用的 CPU 时间比例
	if gcStats.NumGC > 0 && !gcStats.LastGC.IsZero() {
		totalTime := time.Since(gcStats.LastGC)
		if totalTime > 0 && gcStats.PauseTotal > 0 {
			gcPercent := float64(gcStats.PauseTotal) / float64(totalTime) * 100
			c.gcCPUPct.Set(gcPercent)
		}
	}

	// 线程数（注意：Go runtime 暴露的线程数可能不精确，这里使用近似值）
	c.threads.Set(float64(runtime.NumCgoCall()))
}

// collectCPUMetrics 收集 CPU 指标
func (c *MetricsCollector) collectCPUMetrics() {
	// CPU 使用率
	percent, err := cpu.Percent(0, false)
	if err == nil && len(percent) > 0 {
		c.cpuUsage.Set(percent[0])
	}

	// CPU 负载
	load, err := loadAvg()
	if err == nil {
		c.cpuLoad.Set(load)
	}

	// 上下文切换次数
	counts, err := cpu.Counts(false)
	if err == nil {
		c.ctxSwitch.Set(float64(counts))
	}
}

// collectMemoryMetrics 收集内存指标
func (c *MetricsCollector) collectMemoryMetrics() {
	memStat, err := mem.VirtualMemory()
	if err != nil {
		klog.Errorf("Failed to collect memory metrics: %v", err)
		return
	}

	c.memUsed.Set(float64(memStat.Used))
	c.memFree.Set(float64(memStat.Free))
	c.memPercent.Set(memStat.UsedPercent)
}

// collectDiskMetrics 收集磁盘指标
func (c *MetricsCollector) collectDiskMetrics() {
	now := time.Now()
	timeDelta := now.Sub(c.prevTime).Seconds()

	if timeDelta <= 0 {
		return
	}

	// 获取根分区统计信息
	partitions, err := disk.Partitions(false)
	if err != nil {
		klog.Errorf("Failed to get disk partitions: %v", err)
		return
	}

	if len(partitions) > 0 {
		usage, err := disk.Usage(partitions[0].Mountpoint)
		if err != nil {
			klog.Errorf("Failed to get disk usage: %v", err)
			return
		}
		c.diskUsage.Set(usage.UsedPercent)
	}

	// 磁盘 I/O 统计
	ioStats, err := disk.IOCounters()
	if err != nil {
		klog.Errorf("Failed to get disk I/O stats: %v", err)
		return
	}

	var totalIOPS float64
	var totalReadTime, totalWriteTime float64
	var readCount, writeCount uint64
	var totalReadBytesPerSec, totalWriteBytesPerSec float64

	for _, stat := range ioStats {
		// 计算读写吞吐量
		if prevStat, ok := c.prevDiskStats[stat.Name]; ok {
			readBytesDelta := float64(stat.ReadBytes - prevStat.ReadBytes)
			writeBytesDelta := float64(stat.WriteBytes - prevStat.WriteBytes)

			totalReadBytesPerSec += readBytesDelta / timeDelta
			totalWriteBytesPerSec += writeBytesDelta / timeDelta
		}

		totalIOPS += float64(stat.ReadCount + stat.WriteCount)
		totalReadTime += float64(stat.ReadTime)
		totalWriteTime += float64(stat.WriteTime)
		readCount += stat.ReadCount
		writeCount += stat.WriteCount
	}

	c.diskIOPS.Set(totalIOPS)
	c.currentDiskRead = totalReadBytesPerSec
	c.diskRead.Set(totalReadBytesPerSec)
	c.currentDiskWrite = totalWriteBytesPerSec
	c.diskWrite.Set(totalWriteBytesPerSec)

	// 计算平均延迟（单位：毫秒）
	if readCount > 0 {
		readLatency := (totalReadTime / float64(readCount)) / 1000
		c.diskLatency.Observe(readLatency)
	}
	if writeCount > 0 {
		writeLatency := (totalWriteTime / float64(writeCount)) / 1000
		c.diskLatency.Observe(writeLatency)
	}

	// 更新缓存
	c.prevDiskStats = make(map[string]disk.IOCountersStat)
	for _, stat := range ioStats {
		c.prevDiskStats[stat.Name] = stat
	}
}

// collectNetworkMetrics 收集网络指标
func (c *MetricsCollector) collectNetworkMetrics() {
	now := time.Now()
	timeDelta := now.Sub(c.prevTime).Seconds()

	if timeDelta <= 0 {
		return
	}

	// 网络接口统计
	netStats, err := net.IOCounters(true)
	if err != nil {
		klog.Errorf("Failed to collect network metrics: %v", err)
		return
	}

	var totalRxRate, totalTxRate float64
	var totalDropRate, totalRetransmitRate float64
	var connections float64

	for _, stat := range netStats {
		// 计算带宽
		if prevStat, ok := c.prevNetStats[stat.Name]; ok {
			rxDelta := float64(stat.BytesRecv - prevStat.BytesRecv)
			txDelta := float64(stat.BytesSent - prevStat.BytesSent)

			totalRxRate += rxDelta / timeDelta
			totalTxRate += txDelta / timeDelta
		}

		// 丢包率
		totalPackets := float64(stat.PacketsSent + stat.PacketsRecv)
		if totalPackets > 0 {
			dropRate := float64(stat.Dropin+stat.Dropout) / totalPackets * 100
			totalDropRate += dropRate
		}

		// TCP 重传率（在 gopsutil 中可能不直接支持，使用近似值）
		retransmitRate := float64(stat.Errin) / float64(stat.PacketsRecv+1) * 100
		totalRetransmitRate += retransmitRate

		// 连接数（近似值）
		connections += float64(stat.PacketsSent + stat.PacketsRecv) / 100
	}

	// 更新缓存
	c.prevNetStats = make(map[string]net.IOCountersStat)
	for _, stat := range netStats {
		c.prevNetStats[stat.Name] = stat
	}
	c.prevTime = now

	c.currentNetRx = totalRxRate
	c.netBandwidthRx.Set(totalRxRate)
	c.currentNetTx = totalTxRate
	c.netBandwidthTx.Set(totalTxRate)
	c.currentNetDrop = totalDropRate
	c.netDropRate.Set(totalDropRate)
	c.currentTCPRetrans = totalRetransmitRate
	c.tcpRetransmit.Set(totalRetransmitRate)
	c.currentNetConn = connections
	c.netConnections.Set(connections)
}

// loadAvg 获取系统负载平均值
func loadAvg() (float64, error) {
	load, err := loadAvgFromHost()
	if err != nil {
		return 0, err
	}
	return load, nil
}

// loadAvgFromHost 读取 /proc/loadavg 文件获取负载平均值
func loadAvgFromHost() (float64, error) {
	// 读取 /proc/loadavg 文件
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, fmt.Errorf("failed to read /proc/loadavg: %w", err)
	}

	// 解析文件内容
	// 格式: load1 load5 load15 running/total last_pid
	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, fmt.Errorf("invalid /proc/loadavg format")
	}

	// 获取 1 分钟平均负载（第一个字段）
	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse load1: %w", err)
	}

	return load1, nil
}

// GetAllMetrics 获取所有指标，用于 JSON 输出
func (c *MetricsCollector) GetAllMetrics() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	return map[string]interface{}{
		"business": map[string]interface{}{
			"dns_record_count": c.recordCount,
		},
		"go_runtime": map[string]interface{}{
			"goroutines":        runtime.NumGoroutine(),
			"gc_cycles":         debug.GCStats{}.NumGC,
			"heap_alloc_bytes":  c.getHeapAlloc(),
			"heap_sys_bytes":    c.getHeapSys(),
			"threads":           runtime.NumCgoCall(),
			"gc_cpu_percent":    c.getGCPUPct(),
		},
		"cpu": map[string]interface{}{
			"usage_percent":        c.getCPUUsage(),
			"load":                 c.getCPULoad(),
			"context_switches":     c.getContextSwitches(),
		},
		"memory": map[string]interface{}{
			"used_bytes":    c.getMemUsed(),
			"free_bytes":    c.getMemFree(),
			"percent":       c.getMemPercent(),
		},
		"disk": map[string]interface{}{
			"usage_percent":      c.getDiskUsage(),
			"iops":              c.getDiskIOPS(),
			"read_bytes_per_sec": c.getDiskRead(),
			"write_bytes_per_sec": c.getDiskWrite(),
		},
		"network": map[string]interface{}{
			"bandwidth_rx_bytes_per_sec": c.getNetBandwidthRx(),
			"bandwidth_tx_bytes_per_sec": c.getNetBandwidthTx(),
			"connections":                c.getNetConnections(),
			"drop_rate":                  c.getNetDropRate(),
			"tcp_retransmit_rate":        c.getTCPRetransmit(),
		},
	}
}

// 以下 getter 方法用于 JSON 输出
func (c *MetricsCollector) getHeapAlloc() uint64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats.HeapAlloc
}

func (c *MetricsCollector) getHeapSys() uint64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats.HeapSys
}

func (c *MetricsCollector) getGCPUPct() float64 {
	var gcStats debug.GCStats
	gcStats.Pause = make([]time.Duration, 100)
	debug.ReadGCStats(&gcStats)

	totalTime := time.Since(gcStats.LastGC)
	if totalTime > 0 && gcStats.PauseTotal > 0 {
		return float64(gcStats.PauseTotal) / float64(totalTime) * 100
	}
	return 0
}

func (c *MetricsCollector) getCPUUsage() float64 {
	percent, _ := cpu.Percent(0, false)
	if len(percent) > 0 {
		return percent[0]
	}
	return 0
}

func (c *MetricsCollector) getCPULoad() float64 {
	load, _ := loadAvg()
	return load
}

func (c *MetricsCollector) getContextSwitches() float64 {
	counts, _ := cpu.Counts(false)
	return float64(counts)
}

func (c *MetricsCollector) getMemUsed() uint64 {
	memStat, _ := mem.VirtualMemory()
	if memStat != nil {
		return memStat.Used
	}
	return 0
}

func (c *MetricsCollector) getMemFree() uint64 {
	memStat, _ := mem.VirtualMemory()
	if memStat != nil {
		return memStat.Free
	}
	return 0
}

func (c *MetricsCollector) getMemPercent() float64 {
	memStat, _ := mem.VirtualMemory()
	if memStat != nil {
		return memStat.UsedPercent
	}
	return 0
}

func (c *MetricsCollector) getDiskUsage() float64 {
	partitions, _ := disk.Partitions(false)
	if len(partitions) > 0 {
		usage, _ := disk.Usage(partitions[0].Mountpoint)
		if usage != nil {
			return usage.UsedPercent
		}
	}
	return 0
}

func (c *MetricsCollector) getDiskIOPS() float64 {
	ioStats, _ := disk.IOCounters()
	var totalIOPS float64
	for _, stat := range ioStats {
		totalIOPS += float64(stat.ReadCount + stat.WriteCount)
	}
	return totalIOPS
}

func (c *MetricsCollector) getDiskRead() float64 {
	return c.currentDiskRead
}

func (c *MetricsCollector) getDiskWrite() float64 {
	return c.currentDiskWrite
}

func (c *MetricsCollector) getNetBandwidthRx() float64 {
	return c.currentNetRx
}

func (c *MetricsCollector) getNetBandwidthTx() float64 {
	return c.currentNetTx
}

func (c *MetricsCollector) getNetConnections() float64 {
	return c.currentNetConn
}

func (c *MetricsCollector) getNetDropRate() float64 {
	return c.currentNetDrop
}

func (c *MetricsCollector) getTCPRetransmit() float64 {
	return c.currentTCPRetrans
}