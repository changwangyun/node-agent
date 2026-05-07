package utils

import (
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

func GetCPUUsage() (float64, error) {
	percent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return 0, err
	}
	if len(percent) > 0 {
		return percent[0], nil
	}
	return 0, nil
}

func GetCPUPerCore() ([]float64, error) {
	perCore, err := cpu.Percent(0, true)
	if err != nil {
		return nil, err
	}
	return perCore, nil
}

func GetMemoryUsage() (percent float64, usedMB, totalMB uint64) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return 0, 0, 0
	}
	return v.UsedPercent, v.Used / 1024 / 1024, v.Total / 1024 / 1024
}

func GetDiskUsage() (usedGB, totalGB uint64) {
	usage, err := disk.Usage("/")
	if err != nil {
		return 0, 0
	}
	return usage.Used / 1024 / 1024 / 1024, usage.Total / 1024 / 1024 / 1024
}

func GetLoadAvg() (load1, load5, load15 float64) {
	if runtime.GOOS != "linux" {
		return 0, 0, 0
	}
	v, err := load.Avg()
	if err != nil {
		return 0, 0, 0
	}
	return v.Load1, v.Load5, v.Load15
}

func GetUptime() uint64 {
	v, err := host.Uptime()
	if err != nil {
		return 0
	}
	return v
}

var (
	netMu       sync.Mutex
	netPrevRecv uint64
	netPrevSent uint64
	netPrevTime time.Time
	netHasBase  bool
)

func skipNetInterface(name string) bool {
	lower := strings.ToLower(name)
	for _, prefix := range []string{"lo", "docker", "veth", "br-", "virbr", "vnet", "tun", "tap"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func GetNetSpeed() (inSpeed, outSpeed float64) {
	counters, err := net.IOCounters(true)
	if err != nil {
		return -1, -1
	}

	var totalRecv, totalSent uint64
	for _, c := range counters {
		if skipNetInterface(c.Name) {
			continue
		}
		totalRecv += c.BytesRecv
		totalSent += c.BytesSent
	}

	now := time.Now()

	netMu.Lock()
	defer netMu.Unlock()

	if !netHasBase {
		netPrevRecv, netPrevSent, netPrevTime, netHasBase = totalRecv, totalSent, now, true
		return -1, -1
	}

	elapsed := now.Sub(netPrevTime).Seconds()
	if elapsed <= 0 {
		return -1, -1
	}

	if totalRecv < netPrevRecv || totalSent < netPrevSent {
		netPrevRecv, netPrevSent, netPrevTime = totalRecv, totalSent, now
		return -1, -1
	}

	inSpeed = float64(totalRecv-netPrevRecv) / elapsed
	outSpeed = float64(totalSent-netPrevSent) / elapsed

	netPrevRecv, netPrevSent, netPrevTime = totalRecv, totalSent, now
	return inSpeed, outSpeed
}

type GCMetrics struct {
	NumGC       uint32
	LastPauseMS float64
	Goroutines  int
}

func GetGCMetrics() GCMetrics {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	m := GCMetrics{
		Goroutines: runtime.NumGoroutine(),
		NumGC:      ms.NumGC,
	}

	if ms.NumGC > 0 {
		idx := (ms.NumGC - 1) % uint32(len(ms.PauseNs))
		m.LastPauseMS = float64(ms.PauseNs[idx]) / 1e6
	}

	return m
}
