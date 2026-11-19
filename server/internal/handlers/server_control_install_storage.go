package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/ovh-webui/server/internal/numconv"
)

// installOSLocks 按 service_name 分别加锁，防同一台机器并发重装。
// 不同机器互不阻塞。TryLock 失败立即返回 409，不让前端干等。
var installOSLocks sync.Map // service_name → *sync.Mutex

func acquireInstallLock(svc string) (*sync.Mutex, bool) {
	v, _ := installOSLocks.LoadOrStore(svc, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	return mu, mu.TryLock()
}

// reinstallSoftRaidLevels 软 RAID 合法级别
// schema: dedicated.server.reinstall.storage.partitioning.layout.RaidLevelEnum
var reinstallSoftRaidLevels = map[int64]bool{0: true, 1: true, 5: true, 6: true, 7: true, 10: true}

// reinstallHardRaidLevels 硬件 RAID 合法级别
// schema: dedicated.server.reinstall.storage.hardwareRaid.RaidLevelEnum
var reinstallHardRaidLevels = map[int64]bool{0: true, 1: true, 5: true, 6: true, 10: true, 50: true, 60: true}

// reinstallFileSystems 分区文件系统合法取值
// schema: dedicated.server.reinstall.storage.partitioning.layout.FileSystemEnum
var reinstallFileSystems = map[string]bool{
	"btrfs": true, "ext3": true, "ext4": true, "fat16": true, "none": true,
	"ntfs": true, "reiserfs": true, "swap": true, "ufs": true,
	"vmfs5": true, "vmfs6": true, "vmfsl": true, "xfs": true, "zfs": true,
}

// diskSizeToGB 把 complexType.UnitAndValue<long> 归一成 GB。
// schema 只写了 unit 是必填 string、没写枚举，OVH 实际会返回法语缩写(Go/To/Mo)也会返回 GB/TB，
// 两套都认；认不出来的单位宁可放弃自动算容量，也不要按 GB 硬算出一个差 1000 倍的分区大小
func diskSizeToGB(raw interface{}) (int, bool) {
	ds, ok := raw.(map[string]interface{})
	if !ok {
		return 0, false
	}
	v, ok := numconv.ToInt64(ds["value"])
	if !ok {
		return 0, false
	}
	unit, _ := ds["unit"].(string)
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "g", "go", "gb", "gib":
		return int(v), true
	case "m", "mo", "mb", "mib":
		return int(v / 1024), true
	case "t", "to", "tb", "tib":
		return int(v * 1024), true
	case "p", "po", "pb", "pib":
		return int(v * 1024 * 1024), true
	}
	return 0, false
}

// parseRaidLevelValue 兼容前端可能传的 1 / "1" / "raid1" 三种写法
func parseRaidLevelValue(v interface{}) (int64, bool) {
	if v == nil {
		return 0, false
	}
	if n, ok := numconv.ToInt64(v); ok {
		return n, true
	}
	if s, ok := v.(string); ok {
		s = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "raid")
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n, true
		}
	}
	return 0, false
}

// storageError 带稳定错误码和插值参数的存储配置校验错误。
// 前端按 code + params 出当前语言的译文(i18n 插值),error 字段保留中文完整句兜底
type storageError struct {
	msg    string
	code   string
	params map[string]interface{}
}

func (e *storageError) Error() string { return e.msg }

func serr(code string, params map[string]interface{}, msg string) *storageError {
	if params == nil {
		params = map[string]interface{}{}
	}
	return &storageError{msg: msg, code: code, params: params}
}

// checkFSRaidCompat 文件系统 × 软 RAID 级别的不兼容组合。
//
// 取自 OVH 官方分区文档的兼容表(LVM, RAID Levels & Filesystems Compatibility):
//
//	Btrfs / ext4 / XFS   RAID 7  ❌(其余 0/1/5/6/10 都 ✅)
//	ZFS                  RAID 10 ❌(其余 0/1/5/6/7 都 ✅)
//	NTFS                 只有 RAID 1 ✅
//	UFS / VMFS*          任何 RAID 都 ❌
//
// 这些约束只在文档里,schema 的 RaidLevelEnum 是 [0,1,5,6,7,10] 一视同仁,
// 不本地拦的话用户会拿到一句 OVH 的英文错误,不知道是自己选错了组合。
// swap 那一行文档自相矛盾(表格标 RAID 0 可用、RAID 1 不可用,脚注却说"只能设为 1"),
// 所以这里不对 swap 下判断 —— 没有可靠依据的地方不替用户做决定。
func checkFSRaidCompat(fs string, level int64) string {
	switch fs {
	case "btrfs", "ext4", "ext3", "xfs", "reiserfs":
		if level == 7 {
			return "ext4 / XFS / Btrfs 不支持 RAID 7(OVH 分区文档),请改用 0/1/5/6/10"
		}
	case "zfs":
		if level == 10 {
			return "ZFS 不支持 RAID 10(OVH 分区文档),请改用 0/1/5/6/7"
		}
	case "ntfs":
		if level != 1 {
			return "NTFS 只支持 RAID 1(OVH 分区文档)"
		}
	case "ufs", "vmfs5", "vmfs6", "vmfsl":
		return fs + " 不支持任何软 RAID(OVH 分区文档)"
	}
	return ""
}

// validateRaidDiskCount 校验指定 RAID 级别所需的最低物理磁盘数与奇偶约束。
func validateRaidDiskCount(raidLevel int64, diskCount int) string {
	switch raidLevel {
	case 0:
		if diskCount < 1 {
			return "RAID 0 至少需要 1 块物理磁盘"
		}
	case 1:
		if diskCount < 2 {
			return fmt.Sprintf("该磁盘组只有 %d 块物理磁盘，RAID 1 至少需要 2 块磁盘", diskCount)
		}
	case 5:
		if diskCount < 3 {
			return fmt.Sprintf("该磁盘组只有 %d 块物理磁盘，RAID 5 (RAID-Z1) 至少需要 3 块磁盘", diskCount)
		}
	case 6:
		if diskCount < 4 {
			return fmt.Sprintf("该磁盘组只有 %d 块物理磁盘，RAID 6 (RAID-Z2) 至少需要 4 块磁盘", diskCount)
		}
	case 7:
		if diskCount < 5 {
			return fmt.Sprintf("该磁盘组只有 %d 块物理磁盘，RAID 7 (RAID-Z3) 至少需要 5 块磁盘", diskCount)
		}
	case 10:
		if diskCount < 4 || diskCount%2 != 0 {
			return fmt.Sprintf("该磁盘组只有 %d 块物理磁盘，RAID 10 至少需要 4 块且总盘数必须为偶数", diskCount)
		}
	case 50:
		if diskCount < 6 || diskCount%2 != 0 {
			return fmt.Sprintf("该磁盘组只有 %d 块物理磁盘，RAID 50 至少需要 6 块且总盘数必须为偶数", diskCount)
		}
	case 60:
		if diskCount < 8 || diskCount%2 != 0 {
			return fmt.Sprintf("该磁盘组只有 %d 块物理磁盘，RAID 60 至少需要 8 块且总盘数必须为偶数", diskCount)
		}
	}
	return ""
}

// normalizeStorageConfig 把前端的 storageConfig 显式映射成 dedicated.server.reinstall.Storage[]。
// 前端拼硬件 RAID 用的是旧 partitionScheme 的字段名(disks 磁盘编号数组 / mode / name / step)，
// 而 schema 的 HardwareRaid 只有 arrays / disks(数量) / raidLevel / spares，字段名和类型都对不上。
// schema 没写 OVH 会不会忽略未知字段，所以这里只放行 schema 里存在的字段，
// 非法值在后端就拦下来报中文错误，而不是丢给 OVH 返回一句英文错误码
func normalizeStorageConfig(raw interface{}) ([]map[string]interface{}, error) {
	groups, ok := raw.([]interface{})
	if !ok {
		return nil, serr("STORAGE_NOT_ARRAY", nil, "自定义存储配置格式不正确，应为数组")
	}
	out := []map[string]interface{}{}
	// size=0(占满剩余空间)的分区在整份配置里最多一个,跨磁盘组一起数
	fillCount := 0
	for _, gRaw := range groups {
		g, ok := gRaw.(map[string]interface{})
		if !ok {
			return nil, serr("STORAGE_GROUP_NOT_OBJECT", nil, "自定义存储配置格式不正确，磁盘组应为对象")
		}
		entry := map[string]interface{}{}
		// 磁盘组编号从 1 起 —— 官方分区文档:"By default, the OS will be installed
		// on diskGroupId 1",示例里也是 1 / 2。0 是前端"没选磁盘组"的占位值,
		// 发出去等于指定了一个不存在的组。省略即用默认组,这才是文档的语义。
		if v, ok := numconv.ToInt64(g["diskGroupId"]); ok && v > 0 {
			entry["diskGroupId"] = v
		}
		if hrRaw, ok := g["hardwareRaid"].([]interface{}); ok && len(hrRaw) > 0 {
			arrays, err := normalizeHardwareRaidArrays(hrRaw)
			if err != nil {
				return nil, err
			}
			entry["hardwareRaid"] = arrays
		}
		if pRaw, ok := g["partitioning"].(map[string]interface{}); ok {
			part, err := normalizePartitioningGroup(pRaw, &fillCount)
			if err != nil {
				return nil, err
			}
			if len(part) > 0 {
				entry["partitioning"] = part
			}
		}
		// 官方文档 Data erasure:默认所有盘组都会被擦;非安装盘组可以只带 erase:false
		// 声明"保留该组数据"(混合盘机器保数据盘的唯一手段);安装组(带
		// partitioning/hardwareRaid)不允许 erase:false —— 官方明确禁止
		if er, ok := g["erase"].(bool); ok {
			entry["erase"] = er
			if !er && (entry["partitioning"] != nil || entry["hardwareRaid"] != nil) {
				return nil, serr("ERASE_FALSE_ON_INSTALL_GROUP", map[string]interface{}{"gid": g["diskGroupId"]}, "磁盘组 {{gid}} 同时带了存储配置和 erase:false —— 安装盘组必须擦除,erase:false 只能用于非安装盘组")
			}
			if !er && entry["diskGroupId"] == nil {
				return nil, serr("ERASE_FALSE_MISSING_GID", nil, "保留数据的磁盘组必须指定具体的磁盘组编号（diskGroupId > 0）")
			}
		}
		if len(entry) == 0 || (entry["partitioning"] == nil && entry["hardwareRaid"] == nil && entry["erase"] == nil) {
			return nil, serr("STORAGE_GROUP_EMPTY", nil, "自定义存储配置里有磁盘组既没有分区也没有硬件 RAID")
		}
		out = append(out, entry)
	}
	if len(out) == 0 {
		return nil, serr("STORAGE_EMPTY", nil, "自定义存储配置为空")
	}
	if err := validatePreservedStorage(out); err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeHardwareRaidArrays(hrRaw []interface{}) ([]map[string]interface{}, error) {
	arrays := []map[string]interface{}{}
	for _, hRaw := range hrRaw {
		h, ok := hRaw.(map[string]interface{})
		if !ok {
			return nil, serr("HWRAID_BAD_SHAPE", nil, "硬件 RAID 配置格式不正确")
		}
		level, ok := parseRaidLevelValue(h["raidLevel"])
		if !ok {
			level, ok = parseRaidLevelValue(h["mode"])
		}
		if !ok {
			level, ok = parseRaidLevelValue(h["name"])
		}
		if !ok {
			return nil, serr("HWRAID_MISSING_LEVEL", nil, "硬件 RAID 缺少 raidLevel")
		}
		if !reinstallHardRaidLevels[level] {
			return nil, serr("HWRAID_LEVEL_UNSUPPORTED", map[string]interface{}{"level": level}, fmt.Sprintf("硬件 RAID 级别 %d 不受支持，可选：0/1/5/6/10/50/60", level))
		}
		item := map[string]interface{}{"raidLevel": level}
		// schema 的 disks 是"参与阵列的磁盘数量"，前端给的是磁盘编号数组，这里换算成数量
		switch d := h["disks"].(type) {
		case []interface{}:
			if len(d) > 0 {
				item["disks"] = int64(len(d))
			}
		default:
			if n, ok := numconv.ToInt64(h["disks"]); ok && n > 0 {
				item["disks"] = n
			}
		}
		if n, ok := numconv.ToInt64(h["arrays"]); ok && n > 0 {
			item["arrays"] = n
		}
		if n, ok := numconv.ToInt64(h["spares"]); ok && n > 0 {
			item["spares"] = n
		}
		arrays = append(arrays, item)
	}
	return arrays, nil
}

func normalizePartitioningGroup(pRaw map[string]interface{}, fillCount *int) (map[string]interface{}, error) {
	part := map[string]interface{}{}
	if s, ok := pRaw["schemeName"].(string); ok && strings.TrimSpace(s) != "" {
		part["schemeName"] = strings.TrimSpace(s)
	}
	if n, ok := numconv.ToInt64(pRaw["disks"]); ok && n > 0 {
		part["disks"] = n
	}
	lRaw, ok := pRaw["layout"].([]interface{})
	if !ok {
		return part, nil
	}
	layout := []map[string]interface{}{}
	for _, itRaw := range lRaw {
		it, ok := itRaw.(map[string]interface{})
		if !ok {
			return nil, serr("PARTITION_BAD_SHAPE", nil, "分区配置格式不正确")
		}
		fs, _ := it["fileSystem"].(string)
		fs = strings.ToLower(strings.TrimSpace(fs))
		if !reinstallFileSystems[fs] {
			return nil, serr("FS_UNSUPPORTED", map[string]interface{}{"fs": fs}, fmt.Sprintf("不支持的文件系统 %q", fs))
		}
		mp, _ := it["mountPoint"].(string)
		mp = strings.TrimSpace(mp)
		if mp == "" {
			return nil, serr("PARTITION_MISSING_MOUNTPOINT", nil, "分区缺少挂载点")
		}
		size, _ := numconv.ToInt64(it["size"])
		if size < 0 {
			return nil, serr("PARTITION_NEGATIVE_SIZE", map[string]interface{}{"mp": mp}, "分区 {{mp}} 的大小不能为负数")
		}
		// size=0 = 占满剩余空间。官方分区文档:
		// "Up to 1 partition can be configured to fill the remaining space (size 0)"
		if size == 0 {
			*fillCount++
			if *fillCount > 1 {
				return nil, serr("PARTITION_MULTI_FILL", nil, "最多只能有一个分区把大小留空(占满剩余空间),当前有多个,请给其余分区指定大小")
			}
			// 同一份文档明确禁止 swap 占满磁盘:
			// "You have chosen the swap partition to fill the disk ... we disallow this"
			if fs == "swap" {
				return nil, serr("SWAP_CANNOT_FILL", nil, "swap 分区必须指定大小,不能留空占满磁盘(OVH 不允许)")
			}
		}
		// schema: size 是必填 long，0 表示用尽剩余空间
		lay := map[string]interface{}{
			"fileSystem": fs,
			"mountPoint": mp,
			"size":       size,
		}
		if rl, ok := parseRaidLevelValue(it["raidLevel"]); ok {
			if !reinstallSoftRaidLevels[rl] {
				return nil, serr("PARTITION_RAID_UNSUPPORTED", map[string]interface{}{"mp": mp, "level": rl}, "分区 {{mp}} 的软 RAID 级别 {{level}} 不受支持，可选：0/1/5/6/7/10")
			}
			if msg := checkFSRaidCompat(fs, rl); msg != "" {
				return nil, serr("PARTITION_FS_RAID_INCOMPAT", map[string]interface{}{"mp": mp, "reason": msg}, "分区 {{mp}}: {{reason}}")
			}
			lay["raidLevel"] = rl
		}
		if ex, ok := it["extras"].(map[string]interface{}); ok {
			extras := map[string]interface{}{}
			if zp, ok := ex["zp"].(map[string]interface{}); ok {
				if n, ok := zp["name"].(string); ok && n != "" {
					extras["zp"] = map[string]interface{}{"name": n}
				}
			}
			if lv, ok := ex["lv"].(map[string]interface{}); ok {
				if n, ok := lv["name"].(string); ok && n != "" {
					extras["lv"] = map[string]interface{}{"name": n}
				}
			}
			if len(extras) > 0 {
				lay["extras"] = extras
			}
		}
		layout = append(layout, lay)
	}
	if len(layout) > 0 {
		part["layout"] = layout
	}
	return part, nil
}

// A preservation request must never fall back to an unspecified installation
// group, or be contradicted by another entry for the same group.
func validatePreservedStorage(storage []map[string]interface{}) error {
	seen := map[int64]bool{}
	hasKeep, hasInstall, missingID := false, false, false
	for _, group := range storage {
		id, ok := numconv.ToInt64(group["diskGroupId"])
		if !ok || id <= 0 {
			missingID = true
		} else {
			if seen[id] {
				return serr("STORAGE_DUPLICATE_GROUP", nil, "同一磁盘组不能重复配置或同时安装与保留数据")
			}
			seen[id] = true
		}
		if erase, ok := group["erase"].(bool); ok && !erase {
			hasKeep = true
		}
		if group["partitioning"] != nil {
			hasInstall = true
		}
	}
	if hasKeep && (!hasInstall || missingID) {
		return serr("PRESERVE_REQUIRES_INSTALL_GROUP", nil, "保留数据时必须为另一磁盘组明确配置安装分区，不能使用默认磁盘组或仅勾选保留数据")
	}
	return nil
}
