package agent

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// SystemMetrics holds the current system resource usage.
type SystemMetrics struct {
	CPUUsage           float64
	MemoryUsage        int64
	MemoryUsagePercent float64
	Timestamp          time.Time
}

// cpuStat holds CPU statistics from /proc/stat
type cpuStat struct {
	user    uint64
	nice    uint64
	system  uint64
	idle    uint64
	iowait  uint64
	irq     uint64
	softirq uint64
	steal   uint64
}

// GetSystemMetrics collects current system metrics (CPU and memory usage).
// This function is cross-platform with Linux implementation.
func GetSystemMetrics() (*SystemMetrics, error) {
	metrics := &SystemMetrics{
		Timestamp: time.Now(),
	}

	// Get CPU usage
	cpuUsage, err := getCPUUsage()
	if err != nil {
		// Log but don't fail - we can still return memory metrics
		// In production, you might want to log this
	} else {
		metrics.CPUUsage = cpuUsage
	}

	// Get memory usage
	memUsage, memTotal, err := getMemoryUsage()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory usage: %w", err)
	}
	metrics.MemoryUsage = memUsage
	if memTotal > 0 {
		metrics.MemoryUsagePercent = float64(memUsage) / float64(memTotal) * 100
	}

	return metrics, nil
}

// getCPUUsage calculates CPU usage percentage over a short sampling period.
// Returns a value between 0-100 representing the percentage of CPU time used.
func getCPUUsage() (float64, error) {
	if runtime.GOOS != "linux" {
		// For non-Linux platforms, return 0 for now
		// Could implement platform-specific collection later
		return 0, nil
	}

	// Read initial CPU stats
	stat1, err := readCPUStat()
	if err != nil {
		return 0, err
	}

	// Wait for a short period to measure usage
	time.Sleep(100 * time.Millisecond)

	// Read CPU stats again
	stat2, err := readCPUStat()
	if err != nil {
		return 0, err
	}

	// Calculate the difference
	idle := float64(stat2.idle - stat1.idle)
	total := float64((stat2.user + stat2.nice + stat2.system + stat2.idle + stat2.iowait + stat2.irq + stat2.softirq + stat2.steal) -
		(stat1.user + stat1.nice + stat1.system + stat1.idle + stat1.iowait + stat1.irq + stat1.softirq + stat1.steal))

	if total == 0 {
		return 0, nil
	}

	// CPU usage = (total - idle) / total * 100
	usage := (total - idle) / total * 100
	return usage, nil
}

// readCPUStat reads CPU statistics from /proc/stat
func readCPUStat() (*cpuStat, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return nil, fmt.Errorf("failed to read /proc/stat")
	}

	line := scanner.Text()
	fields := strings.Fields(line)
	if len(fields) < 8 || fields[0] != "cpu" {
		return nil, fmt.Errorf("invalid /proc/stat format")
	}

	stat := &cpuStat{}
	values := make([]uint64, 8)
	for i := 0; i < 8; i++ {
		values[i], err = strconv.ParseUint(fields[i+1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CPU stat: %w", err)
		}
	}

	stat.user = values[0]
	stat.nice = values[1]
	stat.system = values[2]
	stat.idle = values[3]
	stat.iowait = values[4]
	stat.irq = values[5]
	stat.softirq = values[6]
	stat.steal = values[7]

	return stat, nil
}

// getMemoryUsage reads memory usage from /proc/meminfo.
// Returns used memory in bytes and total memory in bytes.
func getMemoryUsage() (used int64, total int64, err error) {
	if runtime.GOOS != "linux" {
		// For non-Linux platforms, return 0 for now
		return 0, 0, nil
	}

	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	var memTotal, memFree, buffers, cached int64

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		key := strings.TrimSuffix(fields[0], ":")
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}

		// Values in /proc/meminfo are in kB, convert to bytes
		value *= 1024

		switch key {
		case "MemTotal":
			memTotal = value
		case "MemFree":
			memFree = value
		case "Buffers":
			buffers = value
		case "Cached":
			cached = value
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, 0, err
	}

	// Used memory = Total - Free - Buffers - Cached
	// This gives us a more accurate representation of actually used memory
	memUsed := memTotal - memFree - buffers - cached

	return memUsed, memTotal, nil
}
