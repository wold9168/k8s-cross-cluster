package metrics

import (
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
	"k8s.io/klog/v2"
)

// BaseCollector 收集所有系统级通用指标（Go runtime、CPU、memory、disk、network）。
// 实现 prometheus.Collector，供 sidecar 组合使用。
type BaseCollector struct {
	// Go 运行时指标
	Goroutines prometheus.Gauge
	GcCycles   prometheus.Gauge
	GcDuration prometheus.Summary
	HeapAlloc  prometheus.Gauge
	HeapSys    prometheus.Gauge
	GcCPUPct   prometheus.Gauge
	Threads    prometheus.Gauge

	// CPU 指标
	CPUUsage  prometheus.Gauge
	CPULoad   prometheus.Gauge
	CtxSwitch prometheus.Gauge

	// 内存指标
	MemUsed    prometheus.Gauge
	MemFree    prometheus.Gauge
	MemPercent prometheus.Gauge

	// 磁盘指标
	DiskUsage   prometheus.Gauge
	DiskIOPS    prometheus.Gauge
	DiskLatency prometheus.Summary
	DiskRead    prometheus.Gauge
	DiskWrite   prometheus.Gauge

	// 网络指标
	NetBwRx       prometheus.Gauge
	NetBwTx       prometheus.Gauge
	NetConn       prometheus.Gauge
	NetDropRate   prometheus.Gauge
	TCPRetransmit prometheus.Gauge

	// 用于计算变化率的缓存
	prevNetStats  map[string]net.IOCountersStat
	prevDiskStats map[string]disk.IOCountersStat
	prevTime      time.Time

	// 供 JSON 输出使用的当前采样值
	CurrentDiskRead   float64
	CurrentDiskWrite  float64
	CurrentNetRx      float64
	CurrentNetTx      float64
	CurrentNetConn    float64
	CurrentNetDrop    float64
	CurrentTCPRetrans float64

	mu sync.Mutex
}

// NewBaseCollector 创建新的基础指标收集器
func NewBaseCollector() *BaseCollector {
	c := &BaseCollector{
		prevNetStats:  make(map[string]net.IOCountersStat),
		prevDiskStats: make(map[string]disk.IOCountersStat),
		prevTime:      time.Now(),
	}
	c.initMetrics()
	return c
}

// initMetrics 初始化所有 Prometheus 指标（统一采用 _total 后缀命名）
func (c *BaseCollector) initMetrics() {
	// Go 运行时指标
	c.Goroutines = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_goroutines",
		Help: "Number of goroutines that currently exist",
	})
	c.GcCycles = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_gc_cycles_total",
		Help: "Total number of GC cycles completed",
	})
	c.GcDuration = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "go_gc_duration_seconds",
		Help: "Time spent in GC since program start in quantiles",
	})
	c.HeapAlloc = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_heap_alloc_bytes",
		Help: "Bytes of allocated heap objects",
	})
	c.HeapSys = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_heap_sys_bytes",
		Help: "Bytes of heap memory obtained from the OS",
	})
	c.GcCPUPct = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_gc_cpu_percent",
		Help: "Percentage of CPU time used in garbage collection",
	})
	c.Threads = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "go_threads",
		Help: "Number of OS threads created by the process",
	})

	// CPU 指标
	c.CPUUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_usage_percent",
		Help: "CPU usage percentage",
	})
	c.CPULoad = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_load",
		Help: "System load average",
	})
	c.CtxSwitch = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cpu_context_switches_total",
		Help: "Total number of context switches",
	})

	// 内存指标
	c.MemUsed = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "memory_used_bytes",
		Help: "Memory used in bytes",
	})
	c.MemFree = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "memory_free_bytes",
		Help: "Memory free in bytes",
	})
	c.MemPercent = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "memory_percent",
		Help: "Memory usage percentage",
	})

	// 磁盘指标
	c.DiskUsage = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "disk_usage_percent",
		Help: "Disk usage percentage",
	})
	c.DiskIOPS = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "disk_iops_total",
		Help: "Total disk I/O operations per second",
	})
	c.DiskLatency = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: "disk_latency_seconds",
		Help: "Disk read/write latency in quantiles",
	})
	c.DiskRead = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "disk_read_bytes_per_sec",
		Help: "Disk read throughput in bytes per second",
	})
	c.DiskWrite = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "disk_write_bytes_per_sec",
		Help: "Disk write throughput in bytes per second",
	})

	// 网络指标
	c.NetBwRx = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "network_bandwidth_rx_bytes_per_sec",
		Help: "Network receive bandwidth in bytes per second",
	})
	c.NetBwTx = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "network_bandwidth_tx_bytes_per_sec",
		Help: "Network transmit bandwidth in bytes per second",
	})
	c.NetConn = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "network_connections",
		Help: "Number of network connections",
	})
	c.NetDropRate = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "network_drop_rate",
		Help: "Network packet drop rate",
	})
	c.TCPRetransmit = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tcp_retransmit_rate",
		Help: "TCP segment retransmission rate",
	})
}

// Describe 实现 prometheus.Collector 接口
func (c *BaseCollector) Describe(ch chan<- *prometheus.Desc) {
	c.Goroutines.Describe(ch)
	c.GcCycles.Describe(ch)
	c.GcDuration.Describe(ch)
	c.HeapAlloc.Describe(ch)
	c.HeapSys.Describe(ch)
	c.GcCPUPct.Describe(ch)
	c.Threads.Describe(ch)
	c.CPUUsage.Describe(ch)
	c.CPULoad.Describe(ch)
	c.CtxSwitch.Describe(ch)
	c.MemUsed.Describe(ch)
	c.MemFree.Describe(ch)
	c.MemPercent.Describe(ch)
	c.DiskUsage.Describe(ch)
	c.DiskIOPS.Describe(ch)
	c.DiskLatency.Describe(ch)
	c.DiskRead.Describe(ch)
	c.DiskWrite.Describe(ch)
	c.NetBwRx.Describe(ch)
	c.NetBwTx.Describe(ch)
	c.NetConn.Describe(ch)
	c.NetDropRate.Describe(ch)
	c.TCPRetransmit.Describe(ch)
}

// Collect 实现 prometheus.Collector 接口
func (c *BaseCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.collectGoRuntimeMetrics()
	c.collectCPUMetrics()
	c.collectMemoryMetrics()
	c.collectDiskMetrics()
	c.collectNetworkMetrics()

	c.Goroutines.Collect(ch)
	c.GcCycles.Collect(ch)
	c.GcDuration.Collect(ch)
	c.HeapAlloc.Collect(ch)
	c.HeapSys.Collect(ch)
	c.GcCPUPct.Collect(ch)
	c.Threads.Collect(ch)
	c.CPUUsage.Collect(ch)
	c.CPULoad.Collect(ch)
	c.CtxSwitch.Collect(ch)
	c.MemUsed.Collect(ch)
	c.MemFree.Collect(ch)
	c.MemPercent.Collect(ch)
	c.DiskUsage.Collect(ch)
	c.DiskIOPS.Collect(ch)
	c.DiskLatency.Collect(ch)
	c.DiskRead.Collect(ch)
	c.DiskWrite.Collect(ch)
	c.NetBwRx.Collect(ch)
	c.NetBwTx.Collect(ch)
	c.NetConn.Collect(ch)
	c.NetDropRate.Collect(ch)
	c.TCPRetransmit.Collect(ch)
}

// GetAllMetrics 返回 JSON 格式的所有系统级指标
func (c *BaseCollector) GetAllMetrics() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	return map[string]interface{}{
		"go_runtime": map[string]interface{}{
			"goroutines":       runtime.NumGoroutine(),
			"gc_cycles":        c.getGCCycles(),
			"heap_alloc_bytes": c.getHeapAlloc(),
			"heap_sys_bytes":   c.getHeapSys(),
			"threads":          c.getThreadCount(),
			"gc_cpu_percent":   c.getGCPUPct(),
		},
		"cpu": map[string]interface{}{
			"usage_percent":    c.getCPUUsage(),
			"load":             c.getCPULoad(),
			"context_switches": c.getContextSwitches(),
		},
		"memory": map[string]interface{}{
			"used_bytes": c.getMemUsed(),
			"free_bytes": c.getMemFree(),
			"percent":    c.getMemPercent(),
		},
		"disk": map[string]interface{}{
			"usage_percent":      c.getDiskUsage(),
			"iops":               c.getDiskIOPS(),
			"read_bytes_per_sec": c.CurrentDiskRead,
			"write_bytes_per_sec": c.CurrentDiskWrite,
		},
		"network": map[string]interface{}{
			"bandwidth_rx_bytes_per_sec": c.CurrentNetRx,
			"bandwidth_tx_bytes_per_sec": c.CurrentNetTx,
			"connections":                c.CurrentNetConn,
			"drop_rate":                  c.CurrentNetDrop,
			"tcp_retransmit_rate":        c.CurrentTCPRetrans,
		},
	}
}

// collectGoRuntimeMetrics 收集 Go 运行时指标
func (c *BaseCollector) collectGoRuntimeMetrics() {
	c.Goroutines.Set(float64(runtime.NumGoroutine()))

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	c.HeapAlloc.Set(float64(memStats.HeapAlloc))
	c.HeapSys.Set(float64(memStats.HeapSys))
	c.GcCycles.Set(float64(memStats.NumGC))

	var gcStats debug.GCStats
	gcStats.Pause = make([]time.Duration, 100)
	debug.ReadGCStats(&gcStats)

	if len(gcStats.Pause) > 0 {
		sort.Slice(gcStats.Pause, func(i, j int) bool {
			return gcStats.Pause[i] < gcStats.Pause[j]
		})
		pauseCount := len(gcStats.Pause)
		c.GcDuration.Observe(float64(gcStats.Pause[pauseCount*50/100].Seconds()))
		c.GcDuration.Observe(float64(gcStats.Pause[pauseCount*90/100].Seconds()))
		c.GcDuration.Observe(float64(gcStats.Pause[pauseCount*99/100].Seconds()))
	}

	if gcStats.NumGC > 0 && !gcStats.LastGC.IsZero() {
		totalTime := time.Since(gcStats.LastGC)
		if totalTime > 0 && gcStats.PauseTotal > 0 {
			gcPercent := float64(gcStats.PauseTotal) / float64(totalTime) * 100
			c.GcCPUPct.Set(gcPercent)
		}
	}

	c.Threads.Set(float64(getProcessThreadCount()))
}

// getProcessThreadCount 获取当前进程的 OS 线程数，使用 gopsutil/process
func getProcessThreadCount() int32 {
	p, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return -1
	}
	n, err := p.NumThreads()
	if err != nil {
		return -1
	}
	return n
}

// collectCPUMetrics 收集 CPU 指标
func (c *BaseCollector) collectCPUMetrics() {
	percent, err := cpu.Percent(0, false)
	if err == nil && len(percent) > 0 {
		c.CPUUsage.Set(percent[0])
	}

	load, err := loadAvgFromHost()
	if err == nil {
		c.CPULoad.Set(load)
	}

	counts, err := cpu.Counts(false)
	if err == nil {
		c.CtxSwitch.Set(float64(counts))
	}
}

// collectMemoryMetrics 收集内存指标
func (c *BaseCollector) collectMemoryMetrics() {
	memStat, err := mem.VirtualMemory()
	if err != nil {
		klog.Errorf("Failed to collect memory metrics: %v", err)
		return
	}
	c.MemUsed.Set(float64(memStat.Used))
	c.MemFree.Set(float64(memStat.Free))
	c.MemPercent.Set(memStat.UsedPercent)
}

// collectDiskMetrics 收集磁盘指标
func (c *BaseCollector) collectDiskMetrics() {
	now := time.Now()
	timeDelta := now.Sub(c.prevTime).Seconds()
	if timeDelta <= 0 {
		return
	}

	partitions, err := disk.Partitions(false)
	if err != nil {
		klog.Errorf("Failed to get disk partitions: %v", err)
	} else if len(partitions) > 0 {
		usage, err := disk.Usage(partitions[0].Mountpoint)
		if err != nil {
			klog.Errorf("Failed to get disk usage: %v", err)
		} else {
			c.DiskUsage.Set(usage.UsedPercent)
		}
	}

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

	c.DiskIOPS.Set(totalIOPS)
	c.CurrentDiskRead = totalReadBytesPerSec
	c.DiskRead.Set(totalReadBytesPerSec)
	c.CurrentDiskWrite = totalWriteBytesPerSec
	c.DiskWrite.Set(totalWriteBytesPerSec)

	if readCount > 0 {
		readLatency := (totalReadTime / float64(readCount)) / 1000
		c.DiskLatency.Observe(readLatency)
	}
	if writeCount > 0 {
		writeLatency := (totalWriteTime / float64(writeCount)) / 1000
		c.DiskLatency.Observe(writeLatency)
	}

	c.prevDiskStats = make(map[string]disk.IOCountersStat)
	for _, stat := range ioStats {
		c.prevDiskStats[stat.Name] = stat
	}
}

// collectNetworkMetrics 收集网络指标
func (c *BaseCollector) collectNetworkMetrics() {
	now := time.Now()
	timeDelta := now.Sub(c.prevTime).Seconds()
	if timeDelta <= 0 {
		return
	}

	netStats, err := net.IOCounters(true)
	if err != nil {
		klog.Errorf("Failed to collect network metrics: %v", err)
		return
	}

	var totalRxRate, totalTxRate float64
	var totalDropRate, totalRetransmitRate float64
	var connections float64

	for _, stat := range netStats {
		if prevStat, ok := c.prevNetStats[stat.Name]; ok {
			rxDelta := float64(stat.BytesRecv - prevStat.BytesRecv)
			txDelta := float64(stat.BytesSent - prevStat.BytesSent)
			totalRxRate += rxDelta / timeDelta
			totalTxRate += txDelta / timeDelta
		}

		totalPackets := float64(stat.PacketsSent + stat.PacketsRecv)
		if totalPackets > 0 {
			dropRate := float64(stat.Dropin+stat.Dropout) / totalPackets * 100
			totalDropRate += dropRate
		}

		retransmitRate := float64(stat.Errin) / float64(stat.PacketsRecv+1) * 100
		totalRetransmitRate += retransmitRate

		connections += float64(stat.PacketsSent+stat.PacketsRecv) / 100
	}

	c.prevNetStats = make(map[string]net.IOCountersStat)
	for _, stat := range netStats {
		c.prevNetStats[stat.Name] = stat
	}
	c.prevTime = now

	c.CurrentNetRx = totalRxRate
	c.NetBwRx.Set(totalRxRate)
	c.CurrentNetTx = totalTxRate
	c.NetBwTx.Set(totalTxRate)
	c.CurrentNetDrop = totalDropRate
	c.NetDropRate.Set(totalDropRate)
	c.CurrentTCPRetrans = totalRetransmitRate
	c.TCPRetransmit.Set(totalRetransmitRate)
	c.CurrentNetConn = connections
	c.NetConn.Set(connections)
}

// getter 方法供 JSON 输出使用

func (c *BaseCollector) getGCCycles() uint32 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats.NumGC
}

func (c *BaseCollector) getHeapAlloc() uint64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats.HeapAlloc
}

func (c *BaseCollector) getHeapSys() uint64 {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return memStats.HeapSys
}

func (c *BaseCollector) getThreadCount() int32 {
	return getProcessThreadCount()
}

func (c *BaseCollector) getGCPUPct() float64 {
	var gcStats debug.GCStats
	gcStats.Pause = make([]time.Duration, 100)
	debug.ReadGCStats(&gcStats)

	if gcStats.NumGC > 0 && !gcStats.LastGC.IsZero() {
		totalTime := time.Since(gcStats.LastGC)
		if totalTime > 0 && gcStats.PauseTotal > 0 {
			return float64(gcStats.PauseTotal) / float64(totalTime) * 100
		}
	}
	return 0
}

func (c *BaseCollector) getCPUUsage() float64 {
	percent, _ := cpu.Percent(0, false)
	if len(percent) > 0 {
		return percent[0]
	}
	return 0
}

func (c *BaseCollector) getCPULoad() float64 {
	load, _ := loadAvgFromHost()
	return load
}

func (c *BaseCollector) getContextSwitches() float64 {
	counts, _ := cpu.Counts(false)
	return float64(counts)
}

func (c *BaseCollector) getMemUsed() uint64 {
	memStat, _ := mem.VirtualMemory()
	if memStat != nil {
		return memStat.Used
	}
	return 0
}

func (c *BaseCollector) getMemFree() uint64 {
	memStat, _ := mem.VirtualMemory()
	if memStat != nil {
		return memStat.Free
	}
	return 0
}

func (c *BaseCollector) getMemPercent() float64 {
	memStat, _ := mem.VirtualMemory()
	if memStat != nil {
		return memStat.UsedPercent
	}
	return 0
}

func (c *BaseCollector) getDiskUsage() float64 {
	partitions, _ := disk.Partitions(false)
	if len(partitions) > 0 {
		usage, _ := disk.Usage(partitions[0].Mountpoint)
		if usage != nil {
			return usage.UsedPercent
		}
	}
	return 0
}

func (c *BaseCollector) getDiskIOPS() float64 {
	ioStats, _ := disk.IOCounters()
	var totalIOPS float64
	for _, stat := range ioStats {
		totalIOPS += float64(stat.ReadCount + stat.WriteCount)
	}
	return totalIOPS
}
