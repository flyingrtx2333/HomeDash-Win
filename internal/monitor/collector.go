package monitor

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// SystemStats 系统状态信息
type SystemStats struct {
	CPU     CPUStats     `json:"cpu"`
	Memory  MemoryStats  `json:"memory"`
	GPU     GPUStats     `json:"gpu"`
	Network NetworkStats `json:"network"`
	Disks   []DiskStats  `json:"disks"`
	Time    int64        `json:"time"`
}

// NetworkStats 网络流量信息
type NetworkStats struct {
	BytesSent uint64 `json:"bytesSent"` // 累计发送字节
	BytesRecv uint64 `json:"bytesRecv"` // 累计接收字节
	SpeedSent uint64 `json:"speedSent"` // 发送速率 (bytes/s)
	SpeedRecv uint64 `json:"speedRecv"` // 接收速率 (bytes/s)
}

// CPUStats CPU 信息
type CPUStats struct {
	Usage       float64  `json:"usage"`       // 总使用率
	CoreUsage   []float64 `json:"coreUsage"`  // 每核使用率
	ModelName   string   `json:"modelName"`   // CPU 型号
	Cores       int      `json:"cores"`       // 核心数
	Temperature float64  `json:"temperature"` // 温度（如果可用）
}

// MemoryStats 内存信息
type MemoryStats struct {
	Total       uint64  `json:"total"`       // 总内存（字节）
	Used        uint64  `json:"used"`        // 已用内存
	Available   uint64  `json:"available"`   // 可用内存
	UsedPercent float64 `json:"usedPercent"` // 使用百分比
}

// GPUStats GPU 信息
type GPUStats struct {
	Name        string  `json:"name"`        // GPU 名称
	Usage       float64 `json:"usage"`       // GPU 使用率
	MemoryTotal uint64  `json:"memoryTotal"` // 显存总量（MB）
	MemoryUsed  uint64  `json:"memoryUsed"`  // 已用显存（MB）
	Temperature float64 `json:"temperature"` // GPU 温度
	Available   bool    `json:"available"`   // GPU 是否可用
}

// DiskStats 磁盘信息
type DiskStats struct {
	Device      string  `json:"device"`      // 设备名/盘符
	MountPoint  string  `json:"mountPoint"`  // 挂载点
	Total       uint64  `json:"total"`       // 总容量（字节）
	Used        uint64  `json:"used"`        // 已用容量
	Free        uint64  `json:"free"`        // 可用容量
	UsedPercent float64 `json:"usedPercent"` // 使用百分比
	FSType      string  `json:"fsType"`      // 文件系统类型
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	CPU        float64 `json:"cpu"`
	Memory     uint64  `json:"memory"`     // 内存使用（字节）
	MemPercent float64 `json:"memPercent"` // 内存使用百分比
	Status     string  `json:"status"`
}

// Collector 系统信息采集器
type Collector struct {
	cpuModelName  string
	cpuCores      int
	lastNetStats  net.IOCountersStat
	lastNetTime   time.Time
	cpuInfoCache  cpu.InfoStat // CPU 信息缓存（不变，无需重复获取）
	cpuInfoCached bool
}

// NewCollector 创建采集器
func NewCollector() *Collector {
	c := &Collector{}
	c.initCPUInfo()
	c.initNetInfo()
	return c
}

// initCPUInfo 初始化 CPU 信息（只需获取一次）
func (c *Collector) initCPUInfo() {
	infos, err := cpu.Info()
	if err == nil && len(infos) > 0 {
		c.cpuModelName = infos[0].ModelName
		c.cpuCores = int(infos[0].Cores)
		c.cpuInfoCache = infos[0]
		c.cpuInfoCached = true
	}

	// 获取逻辑核心数（更准确）
	counts, err := cpu.Counts(true)
	if err == nil {
		c.cpuCores = counts
	}
}

// initNetInfo 初始化网络信息（获取初始值用于计算速率）
func (c *Collector) initNetInfo() {
	counters, err := net.IOCounters(false)
	if err == nil && len(counters) > 0 {
		c.lastNetStats = counters[0]
		c.lastNetTime = time.Now()
	}
}

// Collect 采集系统信息
func (c *Collector) Collect() SystemStats {
	stats := SystemStats{
		Time: time.Now().UnixMilli(),
	}

	// 采集 CPU 信息
	stats.CPU = c.collectCPU()

	// 采集内存信息
	stats.Memory = c.collectMemory()

	// 采集 GPU 信息
	stats.GPU = c.collectGPU()

	// 采集网络信息
	stats.Network = c.collectNetwork()

	// 采集磁盘信息
	stats.Disks = c.collectDisks()

	return stats
}

// collectCPU 采集 CPU 信息
func (c *Collector) collectCPU() CPUStats {
	stats := CPUStats{
		ModelName: c.cpuModelName,
		Cores:     c.cpuCores,
	}

	// 获取总使用率
	percentages, err := cpu.Percent(time.Second, false)
	if err == nil && len(percentages) > 0 {
		stats.Usage = percentages[0]
	}

	// 获取每核使用率
	corePercentages, err := cpu.Percent(0, true)
	if err == nil {
		stats.CoreUsage = corePercentages
	}

	// 获取 CPU 温度
	stats.Temperature = GetCPUTemperature()

	return stats
}

// collectMemory 采集内存信息
func (c *Collector) collectMemory() MemoryStats {
	stats := MemoryStats{}

	v, err := mem.VirtualMemory()
	if err == nil {
		stats.Total = v.Total
		stats.Used = v.Used
		stats.Available = v.Available
		stats.UsedPercent = v.UsedPercent
	}

	return stats
}

// collectNetwork 采集网络流量信息
func (c *Collector) collectNetwork() NetworkStats {
	stats := NetworkStats{}

	counters, err := net.IOCounters(false)
	if err != nil || len(counters) == 0 {
		return stats
	}

	current := counters[0]
	now := time.Now()

	// 累计流量
	stats.BytesSent = current.BytesSent
	stats.BytesRecv = current.BytesRecv

	// 计算速率
	elapsed := now.Sub(c.lastNetTime).Seconds()
	if elapsed > 0 && c.lastNetTime.Unix() > 0 {
		stats.SpeedSent = uint64(float64(current.BytesSent-c.lastNetStats.BytesSent) / elapsed)
		stats.SpeedRecv = uint64(float64(current.BytesRecv-c.lastNetStats.BytesRecv) / elapsed)
	}

	// 更新上次记录
	c.lastNetStats = current
	c.lastNetTime = now

	return stats
}

// collectGPU 采集 GPU 信息（通过 nvidia-smi）
func (c *Collector) collectGPU() GPUStats {
	stats := GPUStats{
		Available: false,
	}

	// 尝试使用 nvidia-smi 获取 NVIDIA GPU 信息
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=name,utilization.gpu,memory.total,memory.used,temperature.gpu",
		"--format=csv,noheader,nounits")

	output, err := cmd.Output()
	if err != nil {
		return stats
	}

	// 解析输出
	line := strings.TrimSpace(string(output))
	if line == "" {
		return stats
	}

	// 分割第一行（如果有多个 GPU，只取第一个）
	lines := strings.Split(line, "\n")
	if len(lines) > 0 {
		parts := strings.Split(lines[0], ", ")
		if len(parts) >= 5 {
			stats.Available = true
			stats.Name = strings.TrimSpace(parts[0])

			if usage, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
				stats.Usage = usage
			}
			if memTotal, err := strconv.ParseUint(strings.TrimSpace(parts[2]), 10, 64); err == nil {
				stats.MemoryTotal = memTotal
			}
			if memUsed, err := strconv.ParseUint(strings.TrimSpace(parts[3]), 10, 64); err == nil {
				stats.MemoryUsed = memUsed
			}
			if temp, err := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64); err == nil {
				stats.Temperature = temp
			}
		}
	}

	return stats
}

// collectDisks 采集磁盘信息
func (c *Collector) collectDisks() []DiskStats {
	var stats []DiskStats

	partitions, err := disk.Partitions(false)
	if err != nil {
		return stats
	}

	// 用于去重
	seen := make(map[string]bool)

	for _, p := range partitions {
		// 跳过某些特殊分区
		if strings.HasPrefix(p.Device, "\\\\") {
			continue
		}

		// Windows 下去重（同一个盘符可能出现多次）
		mountPoint := p.Mountpoint
		if seen[mountPoint] {
			continue
		}
		seen[mountPoint] = true

		usage, err := disk.Usage(mountPoint)
		if err != nil {
			continue
		}

		// 跳过容量为 0 的分区
		if usage.Total == 0 {
			continue
		}

		stats = append(stats, DiskStats{
			Device:      p.Device,
			MountPoint:  mountPoint,
			Total:       usage.Total,
			Used:        usage.Used,
			Free:        usage.Free,
			UsedPercent: usage.UsedPercent,
			FSType:      p.Fstype,
		})
	}

	return stats
}

// FormatBytes 格式化字节数为人类可读格式
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// 简化 GPU 名称的辅助函数
func simplifyGPUName(name string) string {
	// 移除常见的冗余前缀
	name = strings.ReplaceAll(name, "NVIDIA ", "")
	name = strings.ReplaceAll(name, "GeForce ", "")
	
	// 使用正则移除多余空格
	re := regexp.MustCompile(`\s+`)
	name = re.ReplaceAllString(name, " ")
	
	return strings.TrimSpace(name)
}

// GetTopProcesses 获取 CPU/内存占用最高的进程
func GetTopProcesses(limit int) []ProcessInfo {
	var processes []ProcessInfo

	procs, err := process.Processes()
	if err != nil {
		return processes
	}

	for _, p := range procs {
		name, err := p.Name()
		if err != nil {
			continue
		}

		cpuPercent, _ := p.CPUPercent()
		memInfo, err := p.MemoryInfo()
		if err != nil {
			continue
		}

		memPercent, _ := p.MemoryPercent()
		status, _ := p.Status()

		// 过滤掉系统进程和空闲进程
		if name == "System Idle Process" || name == "" {
			continue
		}

		processes = append(processes, ProcessInfo{
			PID:        p.Pid,
			Name:       name,
			CPU:        cpuPercent,
			Memory:     memInfo.RSS,
			MemPercent: float64(memPercent),
			Status:     strings.Join(status, ","),
		})
	}

	// 按 CPU 使用率排序
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].CPU > processes[j].CPU
	})

	// 限制返回数量
	if len(processes) > limit {
		processes = processes[:limit]
	}

	return processes
}

// GetCPUTemperature 获取 CPU 温度（Windows 需要管理员权限或特定工具）
func GetCPUTemperature() float64 {
	// Windows 上获取 CPU 温度比较复杂，需要 WMI 或 Open Hardware Monitor
	// 使用 PowerShell 尝试获取（带超时控制）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-Command",
		`Get-WmiObject MSAcpi_ThermalZoneTemperature -Namespace "root/wmi" -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty CurrentTemperature`)

	output, err := cmd.Output()
	if err != nil || ctx.Err() == context.DeadlineExceeded {
		return 0 // 获取失败或超时
	}

	tempStr := strings.TrimSpace(string(output))
	if tempStr == "" {
		return 0
	}

	// WMI 返回的温度是 Kelvin * 10
	temp, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		return 0
	}

	// 转换为摄氏度
	celsius := (temp / 10) - 273.15
	if celsius < 0 || celsius > 150 {
		return 0 // 无效温度
	}

	return celsius
}
