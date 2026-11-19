package price

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"
	"golang.org/x/sync/singleflight"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/orderterm"
	"github.com/ovh-webui/server/internal/ovh"
)

// Result 询价结果
type Result struct {
	Success    bool     `json:"success"`
	PlanCode   string   `json:"planCode,omitempty"`
	Datacenter string   `json:"datacenter,omitempty"`
	Options    []string `json:"options,omitempty"`
	// Degraded:价格算出来了,但购物车没有完全配成用户要的样子(某个非机房必填配置没设上)。
	// 价格仍可展示,但不能拿它当"可以下单"的闸门 —— purchase.go 对同样的配置是 fail-fast 的。
	Degraded       bool       `json:"degraded,omitempty"`
	DegradedReason string     `json:"degradedReason,omitempty"`
	Price          *PriceInfo `json:"price"`
	Error          string     `json:"error,omitempty"`
	// Transient 失败来自超时、网络、408/429 或 5xx 等暂时性错误，不代表配置不能下单。
	// 监控据此区分「没验成」与「明确不可下单」，补货窗口不再因限流/超时丢单。
	Transient bool                   `json:"transient,omitempty"`
	Raw       map[string]interface{} `json:"-"`
}

// transientErr 超时、网络错误、408、429、5xx 视为暂时性；明确的业务 4xx 不是。
func transientErr(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *ovhsdk.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == 408 || apiErr.Code == 429 || apiErr.Code >= 500
	}
	return true
}

type PriceInfo struct {
	// Duration 本次报价对应的购买时长(P1M/P12M...)
	Duration    string                   `json:"duration,omitempty"`
	PricingMode string                   `json:"pricingMode"`
	Prices      map[string]interface{}   `json:"prices"`
	Items       []map[string]interface{} `json:"items"`
}

// internalPriceCache 短 TTL 询价结果缓存。
// 背景:监控每轮对 orderable 机房全量验价(每个 7-9 个串行 OVH 调用),
// 持续有货的订阅按 5 秒一轮就能把账户打进 429 —— 而 429 一来验价失败,
// 自动下单被闸门挡住,等于"监控自己把下单通道刷死"。
// 询价结果(同一配置同一机房)在 20 秒内不会实质变化,复用它即可把
// 每分钟的完整购物车周期降一个数量级。入队时刻的二次验价(monitor 刚验过
// 一遍、batchOrder 又逐条验)同样命中这里,补货瞬间的突发消失。
var internalPriceCache = struct {
	sync.Mutex
	m map[string]cacheEntry
}{m: make(map[string]cacheEntry)}

type cacheEntry struct {
	at     time.Time
	result Result
}

const priceCacheTTL = 20 * time.Second

var priceFlight singleflight.Group

type priceFlightValue struct {
	result    Result
	cartID    string
	cartTaken atomic.Bool
}

func priceCacheKey(accountID, planCode, datacenter string, options []string) string {
	sort.Strings(options)
	apiDC := ovh.ConvertDisplayDCToAPIDC(datacenter)
	return accountID + "|" + planCode + "@" + apiDC + "|" + strings.Join(options, ",")
}

// PeekCached 仅查询内存中的短 TTL 询价缓存，绝不发起任何 OVH 网络请求或建车。
// 适用于监控批量入队等热路径：有缓存直接复用价格，无缓存立即放行不阻塞。
func PeekCached(accountID, planCode, datacenter string, options []string, duration string) (Result, bool) {
	normalized, err := orderterm.Normalize(duration)
	if err != nil {
		return Result{}, false
	}
	key := normalized + "|" + priceCacheKey(accountID, planCode, datacenter, append([]string{}, options...))
	internalPriceCache.Lock()
	defer internalPriceCache.Unlock()
	if e, ok := internalPriceCache.m[key]; ok && time.Since(e.at) < priceCacheTTL {
		return e.result, true
	}
	return Result{}, false
}

// GetInternalCached 带短 TTL 缓存的 GetInternal(监控轮询/批量入队路径用;
// 用户显式点击的询价仍走无缓存的 GetInternal,保证手查永远新鲜)
func GetInternalCached(state *app.State, accountID, planCode, datacenter string, options []string, duration string) Result {
	r, _ := getInternalCached(state, accountID, planCode, datacenter, options, duration, false)
	return r
}

// GetInternalCachedKeepCart 同 GetInternalCached,但询价成功(且未降级、已完成 assign 绑定)时不删购物车,
// 把配好的车号一并返回,调用方可直接拿去当预热车结账 —— 监控发现补货时要先验价,
// 这辆车本来就是按下单的配置装好、绑定过的,删掉再重装一辆纯属浪费抢购时间。
// 命中缓存时没有新车,cartID 为空;调用方不用这辆车时必须自己删掉。
func GetInternalCachedKeepCart(state *app.State, accountID, planCode, datacenter string, options []string, duration string) (Result, string) {
	return getInternalCached(state, accountID, planCode, datacenter, options, duration, true)
}

func getInternalCached(state *app.State, accountID, planCode, datacenter string, options []string, duration string, keepCart bool) (Result, string) {
	normalized, err := orderterm.Normalize(duration)
	if err != nil {
		return Result{Success: false, Error: err.Error()}, ""
	}
	// 购买时长必须进缓存键:月付和年付是两张不同的报价,混用会把年付价当月付价显示
	key := normalized + "|" + priceCacheKey(accountID, planCode, datacenter, append([]string{}, options...))
	internalPriceCache.Lock()
	if e, ok := internalPriceCache.m[key]; ok && time.Since(e.at) < priceCacheTTL {
		internalPriceCache.Unlock()
		return e.result, ""
	}
	internalPriceCache.Unlock()

	val, _, _ := priceFlight.Do(key, func() (interface{}, error) {
		internalPriceCache.Lock()
		if e, ok := internalPriceCache.m[key]; ok && time.Since(e.at) < priceCacheTTL {
			internalPriceCache.Unlock()
			return &priceFlightValue{result: e.result}, nil
		}
		internalPriceCache.Unlock()

		r, cid := getInternal(state, accountID, planCode, datacenter, options, normalized, keepCart)
		// 严禁对失败结果（429/5xx/网络超时/尚未开售等）做负缓存，防止 20s 毒化补货黄金窗口
		if r.Success {
			internalPriceCache.Lock()
			if len(internalPriceCache.m) > 512 {
				for k, e := range internalPriceCache.m {
					if time.Since(e.at) >= priceCacheTTL {
						delete(internalPriceCache.m, k)
					}
				}
			}
			internalPriceCache.m[key] = cacheEntry{at: time.Now(), result: r}
			internalPriceCache.Unlock()
		}
		return &priceFlightValue{result: r, cartID: cid}, nil
	})

	fv := val.(*priceFlightValue)
	if keepCart && fv.cartID != "" && fv.cartTaken.CompareAndSwap(false, true) {
		return fv.result, fv.cartID
	}
	return fv.result, ""
}
