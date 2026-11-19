package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
)

// buildProxmoxZFSStorage 计算并构造 Proxmox 9 + ZFS 预设所需的 storage 配置。
// 校验失败或硬件信息查询失败时直接写入 HTTP 响应并返回 false。
func buildProxmoxZFSStorage(state *app.State, client *ovhsdk.Client, c *gin.Context, svc string, body map[string]interface{}) ([]map[string]interface{}, bool) {
	raidLevel := int64(1)
	if raw, ok := body["zfsRaidLevel"]; ok && raw != nil {
		v, ok := numconv.ToInt64(raw)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "zfsRaidLevel 必须是整数", "code": "E83AB4138"})
			return nil, false
		}
		raidLevel = v
	}
	if !reinstallSoftRaidLevels[raidLevel] {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("RAID 级别 %d 不受支持，可选：0/1/5/6/7/10", raidLevel),
			"code":    "ZFS_RAID_LEVEL_UNSUPPORTED",
			"params":  map[string]interface{}{"level": raidLevel},
		})
		return nil, false
	}
	// OVH 官方分区兼容表明确禁止 ZFS + RAID 10，必须在预设入口拦截
	if msg := checkFSRaidCompat("zfs", raidLevel); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   msg,
			"code":    "PARTITION_FS_RAID_INCOMPAT",
			"params":  map[string]interface{}{"mp": "/", "reason": msg, "level": raidLevel},
		})
		return nil, false
	}
	vzSizeMB := int64(102400)
	if raw, ok := body["zfsVzSize"]; ok && raw != nil {
		v, ok := numconv.ToInt64(raw)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "zfsVzSize 必须是整数(MB)", "code": "E6D5BF3D6"})
			return nil, false
		}
		vzSizeMB = v
	}
	if vzSizeMB < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "/var/lib/vz 容量不能为负数", "code": "E348BDC21"})
		return nil, false
	}
	state.Logger.Info(fmt.Sprintf("🎯 使用 Proxmox 9 + ZFS 根文件系统预设 (RAID%d)", raidLevel), "server_control")

	totalCapacityGB := 480
	if raidLevel == 0 {
		totalCapacityGB = 960
	}
	diskCount := 2
	var diskGroupID interface{}
	hwPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "specifications", "hardware")
	if !ok {
		return nil, false
	}
	var hardware map[string]interface{}
	if err := client.Get(hwPath, &hardware); err != nil {
		state.Logger.Error("获取硬件信息失败，无法安全计算 ZFS 分区容量: "+err.Error(), "server_control")
		respondOVHError(c, err)
		return nil, false
	}

	targetDG, resolvedGID, ok := selectZFSDiskGroup(c, body, hardware)
	if !ok {
		return nil, false
	}
	diskGroupID = resolvedGID

	if targetDG != nil {
		if v, ok := numconv.ToInt64(targetDG["numberOfDisks"]); ok && v > 0 {
			diskCount = int(v)
		}
		if singleDiskGB, ok := diskSizeToGB(targetDG["diskSize"]); ok {
			totalCapacityGB = calcRaidTotalGB(raidLevel, singleDiskGB, diskCount)
			state.Logger.Info(fmt.Sprintf("📊 检测到磁盘: %dx%dGB, RAID%d 总容量: %dGB",
				diskCount, singleDiskGB, raidLevel, totalCapacityGB), "server_control")
		} else {
			state.Logger.Warn(fmt.Sprintf("磁盘容量或单位无法识别，使用默认容量: %dGB", totalCapacityGB), "server_control")
		}
	}

	if diskCount < 2 && raidLevel != 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   fmt.Sprintf("该服务器只有 %d 块磁盘，无法使用 RAID%d，请改用 RAID0", diskCount, raidLevel),
			"code":    "ZFS_SINGLE_DISK_RAID",
			"params":  map[string]interface{}{"count": diskCount, "level": raidLevel},
		})
		return nil, false
	}
	if msg := validateRaidDiskCount(raidLevel, diskCount); msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   msg,
			"code":    "ZFS_INSUFFICIENT_DISKS",
			"params":  map[string]interface{}{"count": diskCount, "level": raidLevel},
		})
		return nil, false
	}

	usableCapacityMB := int64(float64(totalCapacityGB) * 1024 * 0.92)
	const bootSizeMB = int64(1024)
	const swapSizeMB = int64(8192)
	bootSwapMB := bootSizeMB + swapSizeMB
	rootSizeMB := usableCapacityMB - bootSwapMB - vzSizeMB
	// schema 的 layout.size 是必填 long，算成负数会被 OVH 以 PartitionIncompatibleParams 拒掉，
	// 在这里拦下来才能给出用户看得懂的可用范围
	if rootSizeMB <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error": fmt.Sprintf("/var/lib/vz 容量 %dMB 超出可用范围：本机 RAID%d 下可用约 %dMB，扣除 /boot %dMB 和 swap %dMB 后，该值必须小于 %dMB",
				vzSizeMB, raidLevel, usableCapacityMB, bootSizeMB, swapSizeMB, usableCapacityMB-bootSwapMB),
			"code": "ZFS_VZ_OVER_LIMIT",
			"params": map[string]interface{}{
				"vz": vzSizeMB, "level": raidLevel, "usable": usableCapacityMB,
				"boot": bootSizeMB, "swap": swapSizeMB, "max": usableCapacityMB - bootSwapMB,
			},
		})
		return nil, false
	}
	state.Logger.Info(fmt.Sprintf("💾 容量计算: 理论%dGB, 实际可用~%dGB, 根目录%dGB",
		totalCapacityGB, usableCapacityMB/1024, rootSizeMB/1024), "server_control")

	// /boot (ext4) 与 swap 在单盘机器用 RAID 0，多盘机器用 RAID 1 镜像：
	// 绝不透传 RAID 5/6/7（ext4 明令禁止 RAID 7 且引导分区只支持镜像/单盘）
	bootSwapRaidLevel := int64(1)
	if diskCount < 2 {
		bootSwapRaidLevel = 0
	}
	storageEntry := map[string]interface{}{
		"partitioning": map[string]interface{}{
			"layout": []map[string]interface{}{
				{
					"fileSystem": "ext4",
					"mountPoint": "/boot",
					"raidLevel":  bootSwapRaidLevel,
					"size":       bootSizeMB,
				},
				{
					"fileSystem": "swap",
					"mountPoint": "swap",
					"raidLevel":  bootSwapRaidLevel,
					"size":       swapSizeMB,
				},
				{
					"fileSystem": "zfs",
					"mountPoint": "/",
					"raidLevel":  raidLevel,
					"size":       rootSizeMB,
					"extras": map[string]interface{}{
						"zp": map[string]interface{}{"name": "rpool"},
					},
				},
				{
					"fileSystem": "zfs",
					"mountPoint": "/var/lib/vz",
					"raidLevel":  raidLevel,
					"size":       0,
					"extras": map[string]interface{}{
						"zp": map[string]interface{}{"name": "rpool"},
					},
				},
			},
		},
	}
	// diskGroupId 是可空字段，取不到真实 id 就整个省略，让 OVH 用默认磁盘组
	if diskGroupID != nil {
		storageEntry["diskGroupId"] = diskGroupID
	}
	state.Logger.Info(fmt.Sprintf("✅ ZFS 配置: /boot (1GB) + swap (8GB) + / (%dGB) + /var/lib/vz", rootSizeMB/1024), "server_control")
	return []map[string]interface{}{storageEntry}, true
}

func selectZFSDiskGroup(c *gin.Context, body, hardware map[string]interface{}) (map[string]interface{}, interface{}, bool) {
	var explicitGID int64
	hasExplicitGID := false
	if rawGID, exists := body["diskGroupId"]; exists && rawGID != nil {
		if v, ok := numconv.ToInt64(rawGID); ok && v > 0 {
			explicitGID = v
			hasExplicitGID = true
		}
	}
	defaultGID, hasDefaultGID := numconv.ToInt64(hardware["defaultDiskGroupId"])
	if defaultGID <= 0 {
		hasDefaultGID = false
	}
	var diskGroupID interface{}
	wantGID := int64(0)
	if hasExplicitGID {
		wantGID = explicitGID
		diskGroupID = explicitGID
	} else if hasDefaultGID {
		wantGID = defaultGID
		diskGroupID = defaultGID
	}

	// 严禁用 diskGroups[0] 覆盖已识别的 defaultDiskGroupId（防止混合盘机型误格大容量数据盘组）！
	// 按 wantGID 在 diskGroups 中精准匹配目标系统盘组，仅在未提供任何默认/显式组号时才回退 diskGroups[0]。
	var targetDG map[string]interface{}
	if dgs, ok := hardware["diskGroups"].([]interface{}); ok && len(dgs) > 0 {
		if wantGID > 0 {
			for _, item := range dgs {
				dg, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if gid, ok := numconv.ToInt64(dg["diskGroupId"]); ok && gid == wantGID {
					targetDG = dg
					break
				}
			}
			if hasExplicitGID && targetDG == nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   fmt.Sprintf("指定的磁盘组 %d 不存在", explicitGID),
					"code":    "DISK_GROUP_NOT_FOUND",
				})
				return nil, nil, false
			}
		} else if dg0, ok := dgs[0].(map[string]interface{}); ok {
			targetDG = dg0
			if v, ok := numconv.ToInt64(dg0["diskGroupId"]); ok && v > 0 {
				diskGroupID = v
			}
		}
	}
	return targetDG, diskGroupID, true
}

func calcRaidTotalGB(raidLevel int64, singleDiskGB, diskCount int) int {
	switch raidLevel {
	case 0:
		return singleDiskGB * diskCount
	case 1:
		return singleDiskGB
	case 5:
		if diskCount > 1 {
			return singleDiskGB * (diskCount - 1)
		}
	case 6:
		if diskCount > 2 {
			return singleDiskGB * (diskCount - 2)
		}
	case 7:
		if diskCount > 3 {
			return singleDiskGB * (diskCount - 3)
		}
	}
	return singleDiskGB
}
