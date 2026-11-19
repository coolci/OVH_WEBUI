package catalog

import (
	"strings"
)

// dcCityMap 机房城市三字码 → (中文名, 地区)。
// 取值范围以 dedicated.AvailabilityDatacenterEnum 为准(EU/US/CA 三区 schema 完全一致),
// 实测三个站点的 availabilities 都会同时返回短码(gra/bhs/ynm)和长码
// (eu-west-par-a / ca-east-tor-a / us-east-vin-a)。
// 以前只有一张短码表 + code[:3] 截断,长码全被截成 "eu-"/"ca-"/"us-" 落进"未知",
// 巴黎(eu-west-par-*)和多伦多(ca-east-tor-a)这两个三区都在返回的机房永远显示未知。
var dcCityMap = map[string][2]string{
	// 法国
	"gra": {"格拉沃利讷", "法国"},
	"rbx": {"鲁贝", "法国"},
	"sbg": {"斯特拉斯堡", "法国"},
	"par": {"巴黎", "法国"},
	// 德国:OVH 的德国机房在林堡(LIM1/LIM2),对外常按最近的枢纽法兰克福宣传。
	// 老表把 lim 写成"利马索尔/塞浦路斯"是错的 —— OVH 没有塞浦路斯机房。
	"lim": {"林堡", "德国"},
	"fra": {"法兰克福", "德国"},
	// 英国
	"eri": {"埃里斯", "英国"},
	"lon": {"伦敦", "英国"},
	// 其它欧洲
	"waw": {"华沙", "波兰"},
	"mil": {"米兰", "意大利"},
	// 加拿大
	"bhs": {"博阿尔诺", "加拿大"},
	"tor": {"多伦多", "加拿大"},
	"yyz": {"多伦多", "加拿大"},
	// 美国(只有美区站点会返回这两个)
	"vin": {"弗吉尼亚", "美国东部"},
	"hil": {"俄勒冈", "美国西部"},
	// 亚太(欧区/加区目录里都有这些机型,美区站点也会返回)
	"sgp": {"新加坡", "新加坡"},
	"syd": {"悉尼", "澳大利亚"},
	"ynm": {"孟买", "印度"}, // OVH API 用 ynm,前端显示码是 mum
	"mum": {"孟买", "印度"},
}

// dcCountryMap 枚举里还有国家/大区级的取值(au/ca/de/fr/gb/in/pl/sg/us/eu/default),
// 它们不对应具体城市,只能按国家显示。
var dcCountryMap = map[string][2]string{
	"fr":      {"法国", "法国"},
	"de":      {"德国", "德国"},
	"gb":      {"英国", "英国"},
	"pl":      {"波兰", "波兰"},
	"ca":      {"加拿大", "加拿大"},
	"us":      {"美国", "美国"},
	"sg":      {"新加坡", "新加坡"},
	"au":      {"澳大利亚", "澳大利亚"},
	"in":      {"印度", "印度"},
	"eu":      {"欧洲", "欧洲"},
	"default": {"默认机房", "默认"},
}

// dcRegionMap 没有城市段的长码(eu-west-1-a / ca-east-1-a / ap-south-1-a),
// 只能按大区显示,不硬猜具体城市。
var dcRegionMap = map[string][2]string{
	"eu-west-1":  {"欧洲西部", "欧洲"},
	"eu-central": {"欧洲中部", "欧洲"},
	"eu-south":   {"欧洲南部", "欧洲"},
	"ca-east-1":  {"加拿大东部", "加拿大"},
	"us-east-1":  {"美国东部", "美国东部"},
	"us-west-1":  {"美国西部", "美国西部"},
	"ap-south-1": {"亚太南部", "亚太"},
	"ap-east-1":  {"亚太东部", "亚太"},
}

// lookupDCName 把 OVH 返回的机房码解析成 (中文名, 地区)。
// 三区通用,兼容三种写法:短码 gra、带编号 gra1/sgp02、长码 eu-west-par-a。
func lookupDCName(code string) (string, string, bool) {
	c := strings.ToLower(strings.TrimSpace(code))
	if c == "" {
		return "", "", false
	}
	if v, ok := dcCityMap[c]; ok {
		return v[0], v[1], true
	}
	if v, ok := dcCountryMap[c]; ok {
		return v[0], v[1], true
	}
	segs := strings.Split(c, "-")
	// 从后往前扫城市段:长码的首段是大区(ca-east-tor-a 的 "ca"),
	// 从前往后扫会把多伦多认成"加拿大(国家级)",丢掉城市信息。
	for i := len(segs) - 1; i >= 0; i-- {
		if v, ok := dcCityMap[segs[i]]; ok {
			return v[0], v[1], true
		}
	}
	// 没有城市段的长码:去掉结尾的可用区字母再查大区表(eu-west-1-a → eu-west-1)
	if len(segs) > 1 && len(segs[len(segs)-1]) == 1 {
		if v, ok := dcRegionMap[strings.Join(segs[:len(segs)-1], "-")]; ok {
			return v[0], v[1], true
		}
	}
	if len(segs) >= 2 {
		if v, ok := dcRegionMap[strings.Join(segs[:2], "-")]; ok {
			return v[0], v[1], true
		}
	}
	for i := len(segs) - 1; i >= 0; i-- {
		if v, ok := dcCountryMap[segs[i]]; ok {
			return v[0], v[1], true
		}
	}
	// 带编号的短码:gra1 / bhs5 / sgp02
	if len(c) > 3 {
		if v, ok := dcCityMap[c[:3]]; ok {
			return v[0], v[1], true
		}
	}
	return "", "", false
}
