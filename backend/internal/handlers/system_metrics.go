package handlers

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	psCPU "github.com/shirou/gopsutil/v4/cpu"
	psDisk "github.com/shirou/gopsutil/v4/disk"
	psHost "github.com/shirou/gopsutil/v4/host"
	psMem "github.com/shirou/gopsutil/v4/mem"

	"github.com/ovh-webui/server/internal/app"
)

// GetSystemMetrics GET /api/system/metrics
// 返回宿主机当前 CPU / 内存 / 磁盘 + 宿主机基础信息。
// 前端 dashboard 每 2 秒拉一次,做三个圆环。
func GetSystemMetrics(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		// ── CPU ─────────────────────────────────────────────────────
		// percpu=false 取整机平均；interval=0 用上一次采样到现在的差值，几乎瞬时
		cpuPct := 0.0
		if pcts, err := psCPU.Percent(0, false); err == nil && len(pcts) > 0 {
			cpuPct = pcts[0]
		}
		cores := runtime.NumCPU()

		// ── 内存 ────────────────────────────────────────────────────
		var memTotal, memUsed uint64
		var memPct float64
		if vm, err := psMem.VirtualMemory(); err == nil {
			memTotal = vm.Total
			memUsed = vm.Used
			memPct = vm.UsedPercent
		}

		// ── 磁盘 ────────────────────────────────────────────────────
		// Windows 取 C:\，Linux 取 /
		diskPath := "/"
		if runtime.GOOS == "windows" {
			diskPath = "C:\\"
		}
		var diskTotal, diskUsed uint64
		var diskPct float64
		if du, err := psDisk.Usage(diskPath); err == nil {
			diskTotal = du.Total
			diskUsed = du.Used
			diskPct = du.UsedPercent
		}

		// ── 宿主信息 ────────────────────────────────────────────────
		hostname := ""
		platform := runtime.GOOS
		var uptimeSec uint64
		if info, err := psHost.Info(); err == nil {
			hostname = info.Hostname
			if info.Platform != "" {
				platform = info.Platform
			}
			uptimeSec = info.Uptime
		}

		c.JSON(http.StatusOK, gin.H{
			"cpu": gin.H{
				"percent": cpuPct,
				"cores":   cores,
			},
			"memory": gin.H{
				"totalBytes": memTotal,
				"usedBytes":  memUsed,
				"percent":    memPct,
			},
			"disk": gin.H{
				"totalBytes": diskTotal,
				"usedBytes":  diskUsed,
				"percent":    diskPct,
				"path":       diskPath,
			},
			"host": gin.H{
				"hostname":  hostname,
				"platform":  platform,
				"uptimeSec": uptimeSec,
			},
		})
	}
}
