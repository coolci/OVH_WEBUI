package telegram

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const CardDivider = "━━━━━━━━━━━━━━━━━━━━━━━━━"

var (
	memRegex        = regexp.MustCompile(`(?i)ram-(\d+)g(?:-(ecc|noecc))?(?:-(\d+))?`)
	hybridRaidRegex = regexp.MustCompile(`(?i)hybridsoftraid-(\d+)x(\d+)(sa|ssd|hdd|nvme)-(\d+)x(\d+)(sa|ssd|hdd|nvme)`)
	stdRaidRegex    = regexp.MustCompile(`(?i)(?:soft)?raid-(\d+)x(\d+)(sa|ssd|hdd|nvme)`)
	simpleDiskRegex = regexp.MustCompile(`(?i)^(\d+)x(\d+)(sa|ssd|hdd|nvme)`)
	bandwidthRegex  = regexp.MustCompile(`(?i)bandwidth-(\d+)`)
	vrackRegex      = regexp.MustCompile(`(?i)vrack-bandwidth-(\d+)`)
	trafficCombRegex = regexp.MustCompile(`(?i)traffic-(\d+)(tb|gb)-(\d+)`)
	trafficRegex    = regexp.MustCompile(`(?i)traffic-(\d+)(tb|gb)`)
)

func formatDiskType(t string) string {
	switch strings.ToLower(t) {
	case "sa":
		return "SATA"
	case "nvme":
		return "NVMe"
	case "ssd":
		return "SSD"
	case "hdd":
		return "HDD"
	default:
		return strings.ToUpper(t)
	}
}

func formatDiskSize(s string) string {
	n, err := strconv.Atoi(s)
	if err != nil {
		return s + "G"
	}
	if n >= 1000 {
		if n%1000 == 0 {
			return fmt.Sprintf("%dTB", n/1000)
		}
		if n%100 == 0 {
			return fmt.Sprintf("%.1fTB", float64(n)/1000.0)
		}
		return fmt.Sprintf("%.2fTB", float64(n)/1000.0)
	}
	return fmt.Sprintf("%dG", n)
}

// HumanizeMemory 将如 ram-32g-ecc-2133 格式化为人性化显示的 "32G ECC"
func HumanizeMemory(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "?" || v == "N/A" {
		return ""
	}
	m := memRegex.FindStringSubmatch(v)
	if len(m) > 1 {
		size := m[1] + "G"
		ecc := strings.ToLower(m[2])
		if ecc == "ecc" {
			return size + " ECC"
		} else if ecc == "noecc" {
			return size + " Non-ECC"
		}
		return size
	}
	return v
}

// HumanizeStorage 将如 softraid-2x450nvme 格式化为人性化显示的 "2×450G NVMe"
func HumanizeStorage(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "?" || v == "N/A" {
		return ""
	}
	low := strings.ToLower(v)
	if strings.Contains(low, "noraid-0") || strings.Contains(low, "0disk") {
		return "无盘"
	}
	if m := hybridRaidRegex.FindStringSubmatch(v); len(m) > 6 {
		return fmt.Sprintf("%s×%s %s + %s×%s %s",
			m[1], formatDiskSize(m[2]), formatDiskType(m[3]),
			m[4], formatDiskSize(m[5]), formatDiskType(m[6]))
	}
	if m := stdRaidRegex.FindStringSubmatch(v); len(m) > 3 {
		return fmt.Sprintf("%s×%s %s", m[1], formatDiskSize(m[2]), formatDiskType(m[3]))
	}
	if m := simpleDiskRegex.FindStringSubmatch(v); len(m) > 3 {
		return fmt.Sprintf("%s×%s %s", m[1], formatDiskSize(m[2]), formatDiskType(m[3]))
	}
	return v
}

// HumanizeHardware 将内存与硬盘组合格式化，例如 "32G ECC ｜ 2×450G NVMe"
func HumanizeHardware(mem, stor string) string {
	m := HumanizeMemory(mem)
	s := HumanizeStorage(stor)
	if m != "" && s != "" {
		return m + " ｜ " + s
	}
	if m != "" {
		return m
	}
	if s != "" {
		return s
	}
	combined := strings.TrimSpace(mem + " " + stor)
	if combined == "" || combined == "/" {
		return "任意/默认规格"
	}
	return combined
}

// HumanizeOptionCode 将单个 addon planCode 转化为可读标签
func HumanizeOptionCode(code string) string {
	v := strings.TrimSpace(code)
	if v == "" {
		return ""
	}
	low := strings.ToLower(v)
	if strings.HasPrefix(low, "ram-") {
		return HumanizeMemory(v)
	}
	if strings.Contains(low, "raid") || strings.Contains(low, "disk") {
		return HumanizeStorage(v)
	}
	if m := trafficCombRegex.FindStringSubmatch(v); len(m) > 3 {
		speed := m[3]
		spd, err := strconv.Atoi(speed)
		spdStr := speed + "M"
		if err == nil && spd >= 1000 {
			spdStr = fmt.Sprintf("%dG", spd/1000)
		}
		return fmt.Sprintf("%s%s @ %sbps", m[1], strings.ToUpper(m[2]), spdStr)
	}
	if m := trafficRegex.FindStringSubmatch(v); len(m) > 2 {
		return fmt.Sprintf("%s%s 流量", m[1], strings.ToUpper(m[2]))
	}
	if m := vrackRegex.FindStringSubmatch(v); len(m) > 1 {
		speed, _ := strconv.Atoi(m[1])
		if speed >= 1000 {
			return fmt.Sprintf("vRack %d Gbps", speed/1000)
		}
		return fmt.Sprintf("vRack %d Mbps", speed)
	}
	if m := bandwidthRegex.FindStringSubmatch(v); len(m) > 1 {
		speed, _ := strconv.Atoi(m[1])
		if speed >= 1000 {
			return fmt.Sprintf("%d Gbps 带宽", speed/1000)
		}
		return fmt.Sprintf("%d Mbps 带宽", speed)
	}
	if strings.Contains(low, "unlimited") {
		return "不限流量"
	}
	return v
}

// HumanizeOptionCodes 格式化一组选项
func HumanizeOptionCodes(codes []string) string {
	if len(codes) == 0 {
		return "默认配置"
	}
	var parts []string
	for _, c := range codes {
		h := HumanizeOptionCode(c)
		if h != "" {
			parts = append(parts, h)
		}
	}
	if len(parts) == 0 {
		return "默认配置"
	}
	return strings.Join(parts, " ｜ ")
}
