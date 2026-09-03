package telegram

import "strings"

// StandardDCs 与前端 OVH_DATACENTERS 对齐的 12 个机房。
var StandardDCs = []string{"gra", "sbg", "rbx", "bhs", "mum", "waw", "fra", "lon", "hil", "vin", "sgp", "syd"}

func CallbackButton(text, data string) map[string]string {
	return map[string]string{"text": text, "callback_data": data}
}

func InlineKeyboard(rows [][]map[string]string) map[string]interface{} {
	if rows == nil {
		rows = [][]map[string]string{}
	}
	return map[string]interface{}{"inline_keyboard": rows}
}

func ChunkButtons(btns []map[string]string, perRow int) [][]map[string]string {
	if perRow < 1 {
		perRow = 2
	}
	rows := [][]map[string]string{}
	row := []map[string]string{}
	for i, b := range btns {
		row = append(row, b)
		if len(row) >= perRow || i == len(btns)-1 {
			rows = append(rows, row)
			row = nil
		}
	}
	return rows
}

var dcNamesCN = map[string]string{
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
}

func DisplayDC(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if c == "ynm" {
		c = "mum"
	}
	return strings.ToUpper(c)
}

func DisplayDCFull(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if c == "ynm" {
		c = "mum"
	}
	if name, ok := dcNamesCN[c]; ok {
		return name + " (" + strings.ToUpper(c) + ")"
	}
	return strings.ToUpper(c)
}

func NormalizeDC(code string) string {
	c := strings.ToLower(strings.TrimSpace(code))
	if c == "ynm" {
		return "mum"
	}
	return c
}
