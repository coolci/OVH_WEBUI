package monitor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ovh-webui/server/internal/telegram"
)

// 机房代码 → 中文显示。key 一律是「城市段」,长代码由 availabilityDCCity 归一化后再查。
//
// 取值范围来自 dedicated.AvailabilityDatacenterEnum —— 实测 EU / US / CA 三个站点
// 的这份枚举完全一致(48 个取值),里面既有短代码(gra / bhs / vin)也有带可用区的
// 长代码(eu-west-par-a / ca-east-tor-a / ap-southeast-sgp-a)。
// live 数据两种都会出现:三个站点的 availabilities 里都实际返回过
// eu-west-par-a|b|c 和 ca-east-tor-a,而 vin / hil 只在 US 站点出现。
//
// ⚠️ 国别/城市的唯一事实来源是 catalog 包里的 dcCityMap / dcCountryMap
// (服务器列表、前端 web/src/routes/vps-control.tsx 都按那张表显示)。
// 这里之所以还留一份,只是因为 catalog 的 lookupDCName 目前没导出;
// 两张表分歧过一次,后果是同一个机房在 Telegram 通知里叫「意大利·埃里切」、
// 在服务器列表里叫「英国·埃里斯」,用户没法判断到底下单到了哪个国家。
// 改这里必须同步 catalog,catalog 一旦导出 LookupDCName 就把本表删掉。
//
// 已按 catalog 校正过的三处(OVH 官方机房位置):
//
//	eri = 英国 Erith(伦敦东南),不是意大利埃里切
//	lim = 德国 Limburg(林堡),不是波兰利马诺瓦 —— OVH 在波兰只有华沙 WAW
//	bhs = 加拿大 Beauharnois(博阿尔诺),不是博舍维尔
var dcDisplayMapCN = map[string]string{
	"gra":    "🇫🇷 法国·格拉沃利讷",
	"rbx":    "🇫🇷 法国·鲁贝",
	"rbx-hz": "🇫🇷 法国·鲁贝(HZ)",
	"sbg":    "🇫🇷 法国·斯特拉斯堡",
	"par":    "🇫🇷 法国·巴黎",
	"eri":    "🇬🇧 英国·埃里斯",
	"mil":    "🇮🇹 意大利·米兰",
	"lim":    "🇩🇪 德国·林堡",
	"waw":    "🇵🇱 波兰·华沙",
	"fra":    "🇩🇪 德国·法兰克福",
	"lon":    "🇬🇧 英国·伦敦",
	"bhs":    "🇨🇦 加拿大·博阿尔诺",
	"tor":    "🇨🇦 加拿大·多伦多",
	"yyz":    "🇨🇦 加拿大·多伦多",
	"syd":    "🇦🇺 澳大利亚·悉尼",
	"sgp":    "🇸🇬 新加坡",
	"ynm":    "🇮🇳 印度·孟买",
	"mum":    "🇮🇳 印度·孟买",
	"vin":    "🇺🇸 美国·弗吉尼亚",
	"hil":    "🇺🇸 美国·俄勒冈",
	// 枚举里还有国别粒度的取值(不带城市),照样要能显示
	"fr": "🇫🇷 法国", "de": "🇩🇪 德国", "gb": "🇬🇧 英国", "pl": "🇵🇱 波兰",
	"ca": "🇨🇦 加拿大", "us": "🇺🇸 美国", "au": "🇦🇺 澳大利亚",
	"sg": "🇸🇬 新加坡", "in": "🇮🇳 印度",
	// 枚举里还有 eu / default 两个非国家取值,漏了就会显示成裸 "EU" / "DEFAULT"
	"eu": "🇪🇺 欧洲", "default": "默认机房",
}

var dcDisplayShort = map[string]string{
	"gra":    "🇫🇷 Gra",
	"rbx":    "🇫🇷 Rbx",
	"rbx-hz": "🇫🇷 RbxHZ",
	"sbg":    "🇫🇷 Sbg",
	"par":    "🇫🇷 Par",
	"eri":    "🇬🇧 Eri",
	"mil":    "🇮🇹 Mil",
	"lim":    "🇩🇪 Lim",
	"waw":    "🇵🇱 Waw",
	"fra":    "🇩🇪 Fra",
	"lon":    "🇬🇧 Lon",
	"bhs":    "🇨🇦 Bhs",
	"tor":    "🇨🇦 Tor",
	"yyz":    "🇨🇦 Tor",
	"syd":    "🇦🇺 Syd",
	"sgp":    "🇸🇬 Sgp",
	"ynm":    "🇮🇳 Mum",
	"mum":    "🇮🇳 Mum",
	"vin":    "🇺🇸 Vin",
	"hil":    "🇺🇸 Hil",
	"fr":     "🇫🇷 FR", "de": "🇩🇪 DE", "gb": "🇬🇧 GB", "pl": "🇵🇱 PL",
	"ca": "🇨🇦 CA", "us": "🇺🇸 US", "au": "🇦🇺 AU",
	"sg": "🇸🇬 SG", "in": "🇮🇳 IN",
	"eu": "🇪🇺 EU", "default": "默认",
}

// availabilityDCCity 把 dedicated.AvailabilityDatacenterEnum 的取值归一化成城市段 + 可用区。
//
//	"gra"                → ("gra", "")
//	"eu-west-par-b"      → ("par", "B")
//	"ca-east-tor-a"      → ("tor", "A")
//	"ap-southeast-sgp-a" → ("sgp", "A")
//	"rbx-hz"             → ("rbx-hz", "")   两段的先按整体查,查不到再退化
//	"eu-west-1-a"        → ("1", "A")       没有城市名,调用方会退回裸代码
//
// 规则:三段以上且最后一段是单个字母时,那一段是可用区,前一段是城市。
func availabilityDCCity(dc string) (city, az string) {
	d := strings.ToLower(strings.TrimSpace(dc))
	if d == "" {
		return "", ""
	}
	if _, ok := dcDisplayMapCN[d]; ok {
		return d, ""
	}
	parts := strings.Split(d, "-")
	if len(parts) >= 3 {
		last := parts[len(parts)-1]
		if len(last) == 1 && last[0] >= 'a' && last[0] <= 'z' {
			return parts[len(parts)-2], strings.ToUpper(last)
		}
		return last, ""
	}
	return d, ""
}

func dcDisplayCN(dc string) string {
	city, az := availabilityDCCity(dc)
	v, ok := dcDisplayMapCN[city]
	if !ok {
		// 未知代码原样回显:宁可让用户看到裸代码去查,也不要显示一个猜错的城市
		return strings.ToUpper(dc)
	}
	if az != "" {
		return v + " (AZ-" + az + ")"
	}
	return v
}

func dcDisplayShortName(dc string) string {
	city, az := availabilityDCCity(dc)
	v, ok := dcDisplayShort[city]
	if !ok {
		return strings.ToUpper(dc)
	}
	if az != "" {
		return v + "-" + az
	}
	return v
}

func formatConfigSpec(configInfo map[string]interface{}) (summary string, mem string, disk string) {
	if configInfo == nil {
		return "", "", ""
	}
	rawMem, _ := configInfo["memory"].(string)
	rawDisk, _ := configInfo["storage"].(string)
	rawDisplay, _ := configInfo["display"].(string)

	mem = telegram.HumanizeMemory(rawMem)
	if mem == "" {
		mem = rawMem
	}
	disk = telegram.HumanizeStorage(rawDisk)
	if disk == "" {
		disk = rawDisk
	}

	if mem != "" && disk != "" {
		summary = mem + " ｜ " + disk
	} else if mem != "" {
		summary = mem
	} else if disk != "" {
		summary = disk
	} else if rawDisplay != "" {
		parts := strings.Split(rawDisplay, "+")
		if len(parts) == 2 {
			p1 := telegram.HumanizeOptionCode(strings.TrimSpace(parts[0]))
			p2 := telegram.HumanizeOptionCode(strings.TrimSpace(parts[1]))
			if p1 != "" && p2 != "" {
				mem = p1
				disk = p2
				summary = p1 + " ｜ " + p2
			} else {
				summary = rawDisplay
			}
		} else {
			h := telegram.HumanizeOptionCode(rawDisplay)
			if h != "" {
				summary = h
			} else {
				summary = rawDisplay
			}
		}
	}

	if summary == "" {
		if opts, ok := configInfo["options"].([]string); ok && len(opts) > 0 {
			summary = telegram.HumanizeOptionCodes(opts)
		}
	}
	if rawSys, _ := configInfo["system_storage"].(string); rawSys != "" && rawSys != "N/A" {
		sysH := telegram.HumanizeStorage(rawSys)
		if sysH == "" {
			sysH = telegram.HumanizeOptionCode(rawSys)
		}
		if sysH == "" {
			sysH = rawSys
		}
		if summary != "" {
			summary += " ｜ " + sysH
		} else {
			summary = sysH
		}
	}
	if rawGPU, _ := configInfo["gpu"].(string); rawGPU != "" && rawGPU != "N/A" {
		gpuH := telegram.HumanizeOptionCode(rawGPU)
		if gpuH == "" {
			gpuH = rawGPU
		}
		if summary != "" {
			summary += " ｜ " + gpuH
		} else {
			summary = gpuH
		}
	}
	return summary, mem, disk
}

func formatTraceBlock(traceID, configTraceID string) string {
	t1 := strings.TrimSpace(traceID)
	t2 := strings.TrimSpace(configTraceID)
	if t1 == "" && t2 == "" {
		return ""
	}
	shorten := func(id string) string {
		if len(id) > 8 && strings.Contains(id, "-") {
			return id[:8]
		}
		return id
	}
	s1 := shorten(t1)
	s2 := shorten(t2)
	if s1 != "" && s2 != "" {
		if s1 == s2 {
			return "🆔 链路追踪: " + s1
		}
		return "🆔 链路追踪: " + s1 + " / " + s2
	}
	if s1 != "" {
		return "🆔 链路追踪: " + s1
	}
	return "🆔 链路追踪: " + s2
}

// maxOrderButtons 上架通知里最多放几颗一键下单按钮。
// 超了就不按账户铺开 —— 手机上一屏塞十几颗按钮没法用,Telegram 对键盘也有上限。
const maxOrderButtons = 10

// activeQueueCount 这个型号当前有几个进行中的抢购任务。
//
// 用来在上架通知里提醒"你已经在抢这台了"。补货往往连着来好几条通知,
// 用户在手机上对同一台机器按两次按钮是很自然的动作,而那就是两笔真实订单。
// 按钮自己的一次性 claim 只挡得住同一颗按钮按两次,挡不住两条通知各按一次。
func (m *Monitor) activeQueueCount(planCode string) int {
	if m.state == nil {
		return 0
	}
	m.state.QueueMu.Lock()
	defer m.state.QueueMu.Unlock()
	n := 0
	for _, it := range m.state.Queue {
		if it.PlanCode != planCode {
			continue
		}
		if it.Status == "running" || it.Status == "pending" || it.Status == "paused" {
			n++
		}
	}
	return n
}

// productName 通知抬头那一行。
//
// 光一个 planCode(24sk602)在手机上看不出是什么机器,而抢购那一刻用户
// 要在几秒内判断"这是不是我要的那台"。所以把型号名和 CPU 拼出来。
// 目录没加载到时退回 planCode —— 不编,也不留空。
func (m *Monitor) productName(planCode, serverName string) string {
	name := strings.TrimSpace(serverName)
	cpu := ""
	if m.state != nil {
		m.state.ServerPlansMu.RLock()
		for _, p := range m.state.ServerPlans {
			if p.PlanCode == planCode {
				if name == "" {
					name = strings.TrimSpace(p.Name)
				}
				cpu = strings.TrimSpace(p.CPU)
				break
			}
		}
		m.state.ServerPlansMu.RUnlock()
	}
	switch {
	case name != "" && cpu != "" && !strings.Contains(strings.ToLower(name), strings.ToLower(cpu)):
		return name + " | " + cpu
	case name != "":
		return name
	default:
		return planCode
	}
}

// dcButtonLabel 格式化按钮中的机房标签，统一国旗 + 大写 3 位代码 (如 🇩🇪 FRA, 🇫🇷 GRA)
func dcButtonLabel(dc string) string {
	city, az := availabilityDCCity(dc)
	v, ok := dcDisplayShort[city]
	if !ok {
		return strings.ToUpper(dc)
	}
	parts := strings.SplitN(v, " ", 2)
	if len(parts) == 2 {
		v = parts[0] + " " + strings.ToUpper(parts[1])
	}
	if az != "" {
		return v + "-" + az
	}
	return v
}

// dcLine "waw (波兰华沙)"。拿不到中文名就只写代码。
func dcLine(dc string) string {
	code := strings.ToUpper(dc)
	cn := strings.TrimSpace(dcDisplayCN(dc))
	// dcDisplayCN 拿不到时会退回大写代码,那种情况别写成 "WAW (WAW)"
	if cn == "" || strings.EqualFold(cn, dc) {
		return code
	}
	return code + " (" + cn + ")"
}

// availWording 这个机房的可用性该怎么说。
//
// 取的是 OVH 原样返回的值(1H-low / 72H …),它才带着"多久能交付、库存高低"
// 这两个决定要不要立刻下单的信息。没有原值时退回"有货"——
// 那是归一化之后仅剩的事实,不要编一个更具体的说法出来。
func availWording(dcInfo map[string]interface{}) string {
	if raw, ok := dcInfo["raw_status"].(string); ok && raw != "" {
		if w := AvailabilityCN(raw); w != "" {
			return w
		}
	}
	return "有货"
}

// computeFirstMonthTotal 当缺少显式首月总额时，尝试从月费与安装费文本中安全解析并求和
func computeFirstMonthTotal(monthlyText, installText string) string {
	mClean := strings.TrimSpace(strings.TrimSuffix(monthlyText, "/月"))
	iClean := strings.TrimSpace(installText)
	if idx := strings.Index(iClean, "（"); idx != -1 {
		iClean = strings.TrimSpace(iClean[:idx])
	}
	for _, cur := range []string{"€", "£", "₹", "US$", "CA$", "A$", "S$"} {
		if strings.HasPrefix(mClean, cur) && strings.HasPrefix(iClean, cur) {
			mStr := strings.TrimPrefix(mClean, cur)
			iStr := strings.TrimPrefix(iClean, cur)
			mVal, err1 := strconv.ParseFloat(mStr, 64)
			iVal, err2 := strconv.ParseFloat(iStr, 64)
			if err1 == nil && err2 == nil && mVal > 0 && iVal > 0 {
				return fmt.Sprintf("%s%.2f", cur, mVal+iVal)
			}
			break
		}
	}
	return ""
}

// formatInstallFeeLine 格式化安装费显示，附带（一次性）标识，并保留 addon 备注
func formatInstallFeeLine(installText string) string {
	if installText == "" {
		return ""
	}
	if strings.Contains(installText, "（一次性") {
		return installText
	}
	if idx := strings.Index(installText, "（"); idx != -1 {
		prefix := strings.TrimSpace(installText[:idx])
		suffix := installText[idx:]
		return prefix + "（一次性，" + strings.TrimPrefix(suffix, "（")
	}
	return installText + "（一次性）"
}
