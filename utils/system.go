package utils

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
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
