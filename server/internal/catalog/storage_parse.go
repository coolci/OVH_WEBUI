package catalog

import (
	"fmt"
	"regexp"
	"strings"
)

// 介质码匹配：sas 必须在 sa 前，避免 softraid-2x22000sas 被误切成 SA
var reStorageSegment = regexp.MustCompile(`(?i)(\d+)x(\d+)(sas|ssd|nvme|hdd|sa)`)

// FormatMediaLabel 将 OVH 介质缩写转为展示文案
func FormatMediaLabel(code string) string {
	switch strings.ToLower(code) {
	case "sa":
		return "SATA"
	case "sas":
		return "SAS"
	case "nvme":
		return "NVMe"
	case "ssd":
		return "SSD"
	case "hdd":
		return "HDD"
	default:
		return strings.ToUpper(code)
	}
}

func formatStorageSegment(count, sizeGB, media string) string {
	return fmt.Sprintf("%sx %sGB %s", count, sizeGB, FormatMediaLabel(media))
}

// ParseStorageAddonDisplay 从 OVH storage addon planCode 解析可读存储描述。
// 覆盖：softraid/hardraid、hybrid（两段任意介质顺序）、0disk。
// 无法解析时返回空串，由调用方回退原值。
func ParseStorageAddonDisplay(addon string) string {
	if addon == "" {
		return ""
	}
	low := strings.ToLower(strings.TrimSpace(addon))

	// 0 数据盘占位（系统盘另选）
	if strings.Contains(low, "0disk") {
		return "未配置数据盘 (0 disk)"
	}

	segs := reStorageSegment.FindAllStringSubmatch(addon, -1)
	if len(segs) == 0 {
		return ""
	}

	isHybrid := strings.Contains(low, "hybrid")
	if isHybrid && len(segs) >= 2 {
		return fmt.Sprintf("混合RAID %s + %s",
			formatStorageSegment(segs[0][1], segs[0][2], segs[0][3]),
			formatStorageSegment(segs[1][1], segs[1][2], segs[1][3]),
		)
	}
	if isHybrid && len(segs) == 1 {
		return "混合RAID " + formatStorageSegment(segs[0][1], segs[0][2], segs[0][3])
	}

	kind := "SOFTRAID"
	if strings.Contains(low, "hardraid") {
		kind = "HARDRAID"
	} else if strings.HasPrefix(low, "raid-") && !strings.Contains(low, "soft") {
		kind = "RAID"
	} else if strings.Contains(low, "softraid") {
		kind = "SOFTRAID"
	} else if strings.Contains(low, "raid") {
		kind = "RAID"
	}

	// 多段时：若第一段是 system 系统盘，优先用下一段数据盘
	chosen := segs[0]
	if len(segs) > 1 {
		firstTok := strings.ToLower(segs[0][0])
		if i := strings.Index(low, firstTok); i >= 0 {
			end := i + len(firstTok) + 24
			if end > len(low) {
				end = len(low)
			}
			if strings.Contains(low[i:end], "system") {
				chosen = segs[1]
			}
		}
	}

	return fmt.Sprintf("%s %s", kind, formatStorageSegment(chosen[1], chosen[2], chosen[3]))
}
