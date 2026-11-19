package monitor

import "strings"

// PriceCheckErrorKind 验价/装车失败的特征分类
type PriceCheckErrorKind string

const (
	// ErrorKindPreSale 预售等待中：OVH 物理库存或可用性已释出，但商城/销售目录或加车接口尚未解禁
	ErrorKindPreSale PriceCheckErrorKind = "presale_waiting"
	// ErrorKindRegionBlocked 跨区受限：账户所属大区（EU/US/CA）与机型不匹配，无法跨区加购
	ErrorKindRegionBlocked PriceCheckErrorKind = "region_blocked"
	// ErrorKindTransient 瞬时错误：网络波动、限流或购物车服务暂时不可用
	ErrorKindTransient PriceCheckErrorKind = "transient"
	// ErrorKindUnknown 其他错误
	ErrorKindUnknown PriceCheckErrorKind = "unknown"
)

// ClassifyPriceCheckError 分析验价或装车失败的原因特征，区分真正的“预售等待”与“逻辑阻断”
func ClassifyPriceCheckError(errStr string) (PriceCheckErrorKind, string) {
	if errStr == "" {
		return ErrorKindUnknown, ""
	}
	lower := strings.ToLower(errStr)

	// 1. 跨区阻断检测（欧区账户买美区机型、美区账户买欧区等）
	if strings.Contains(errStr, "不在该账户所属子公司") ||
		strings.Contains(errStr, "互不相通") ||
		strings.Contains(errStr, "美区机型带 -us") ||
		(strings.Contains(lower, "subsidiary") && strings.Contains(lower, "mismatch")) {
		return ErrorKindRegionBlocked, "跨区受限（机型与账户所属大区不通）"
	}

	// 2. 预售等待检测（OVH 物理可用性已透出，但商城加车接口尚未解禁）
	if strings.Contains(lower, "plan code not found") ||
		strings.Contains(errStr, "未收录") ||
		strings.Contains(errStr, "未找到 planCode") ||
		strings.Contains(errStr, "销售目录尚未收录") ||
		strings.Contains(lower, "options incompatible") ||
		strings.Contains(lower, "specific options incompatible") {
		return ErrorKindPreSale, "预售等待中（底层库存就绪，商城加车尚未开放）"
	}

	// 3. 瞬时网络/限流/服务降级
	if strings.Contains(lower, "429") ||
		strings.Contains(lower, "too many requests") ||
		strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "deadline exceeded") ||
		strings.Contains(lower, "connection reset") {
		return ErrorKindTransient, "瞬时网络抖动或频控限制"
	}

	return ErrorKindUnknown, errStr
}

