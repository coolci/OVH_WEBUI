package telegram

import (
	"strings"
	"sync"

	"github.com/ovh-webui/server/internal/app"
)

// kvTGActiveCoupon Telegram 侧的「当前默认优惠码」。
//
// 一旦设置，所有 Telegram 交互（监控通知的 1-Click 抢购按钮、全节点入队按钮、
// 向导提交、以及未显式声明 coupon= 的文本指令）都会自动继承该优惠码，
// 极大提升闪售秒杀时的抢购速度，免去在手机键盘上手打优惠码的耗时。
const kvTGActiveCoupon = "telegram_active_coupon"

var (
	memCouponMu sync.RWMutex
	memCoupon   string
)

// ActiveCoupon 获取当前 Telegram 活跃优惠码。
// 优先读 DB，DB 不可用时使用内存兜底。未设置返回空字符串。
func ActiveCoupon(state *app.State) string {
	if state == nil {
		memCouponMu.RLock()
		defer memCouponMu.RUnlock()
		return memCoupon
	}

	var coupon string
	if state.DB != nil {
		_, _ = state.DB.GetKV(kvTGActiveCoupon, &coupon)
	}
	coupon = strings.TrimSpace(coupon)
	if coupon != "" {
		memCouponMu.Lock()
		memCoupon = coupon
		memCouponMu.Unlock()
		return coupon
	}

	memCouponMu.RLock()
	defer memCouponMu.RUnlock()
	return memCoupon
}

// SetActiveCoupon 设置 Telegram 活跃优惠码并持久化落库。
func SetActiveCoupon(state *app.State, coupon string) error {
	coupon = strings.TrimSpace(coupon)
	memCouponMu.Lock()
	memCoupon = coupon
	memCouponMu.Unlock()

	if state != nil && state.DB != nil {
		return state.DB.SetKV(kvTGActiveCoupon, coupon)
	}
	return nil
}

// ClearActiveCoupon 清空当前活跃优惠码。
func ClearActiveCoupon(state *app.State) error {
	return SetActiveCoupon(state, "")
}

