package metrics

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// loadAvgFromHost 读取 /proc/loadavg 文件获取 1 分钟平均负载
func loadAvgFromHost() (float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, fmt.Errorf("failed to read /proc/loadavg: %w", err)
	}

	fields := strings.Fields(string(data))
	if len(fields) < 1 {
		return 0, fmt.Errorf("invalid /proc/loadavg format")
	}

	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse load1: %w", err)
	}

	return load1, nil
}
