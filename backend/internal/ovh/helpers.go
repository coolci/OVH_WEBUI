package ovh

import "strings"

// ConvertDisplayDCToAPIDC 将前端显示的数据中心代码转换为 OVH API 代码
// 对应 Python: _convert_display_dc_to_api_dc
func ConvertDisplayDCToAPIDC(datacenter string) string {
	if datacenter == "" {
		return "gra"
	}
	dcMap := map[string]string{
		"mum": "ynm", // 孟买：前端 mum，OVH API 用 ynm
	}
	lower := strings.ToLower(datacenter)
	if v, ok := dcMap[lower]; ok {
		return v
	}
	return lower
}

// RegionForDC 根据 dc 返回区域（与 purchase_server 中逻辑一致）
func RegionForDC(dc string) string {
	dcLower := strings.ToLower(dc)
	eu := []string{"gra", "rbx", "sbg", "eri", "lim", "waw", "par", "fra", "lon"}
	canada := []string{"bhs"}
	us := []string{"vin", "hil"}
	apac := []string{"syd", "sgp", "ynm"}

	for _, p := range eu {
		if strings.HasPrefix(dcLower, p) {
			return "europe"
		}
	}
	for _, p := range canada {
		if strings.HasPrefix(dcLower, p) {
			return "canada"
		}
	}
	for _, p := range us {
		if strings.HasPrefix(dcLower, p) {
			return "usa"
		}
	}
	for _, p := range apac {
		if strings.HasPrefix(dcLower, p) {
			return "apac"
		}
	}
	return ""
}
