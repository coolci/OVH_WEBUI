package purchase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/orderterm"
	"github.com/ovh-webui/server/internal/ovh"
)

// PrewarmedCart 预装车：在库存探测等待阶段提前在 OVH 侧组装好的满配就绪购物车。
// 包含 base item、机房、操作系统、区域、硬件选件及优惠码。
// 当探测器发现有货时，直接一击发向 checkout，将抢购装车耗时彻底降为 0 秒。
type PrewarmedCart struct {
	CartID     string
	AccountID  string
	PlanCode   string
	Datacenter string
	Options    []string
	Coupon     string
	// Duration 购物车里基础商品与配件的购买时长;空值按 P1M 处理
	Duration  string
	CreatedAt time.Time
}

var (
	prewarmMu    sync.Mutex
	prewarmCarts = map[string]*PrewarmedCart{}
	cartTTL      = 90 * time.Minute // OVH 购物车有效期通常为 2 小时，保守设为 90 分钟
)

// 购买时长是键的一部分:月付预热车绝不能被年付任务直接结账(反之亦然)。
func prewarmKey(accountID, planCode, datacenter string, options []string, coupon, duration string) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		accountID,
		planCode,
		strings.ToLower(datacenter),
		fingerprint(options),
		strings.TrimSpace(coupon),
		normalizedDuration(duration),
	)
}

// normalizedDuration 规整购买时长;非法值原样保留,保证它不会和任何合法周期撞键。
func normalizedDuration(raw string) string {
	if d, err := orderterm.Normalize(raw); err == nil {
		return d
	}
	return "invalid:" + raw
}

// TakePrewarmedCart 获取并消费已就绪的预装车。一旦取出，从池中移除。
func TakePrewarmedCart(accountID, planCode, datacenter string, options []string, coupon, duration string) *PrewarmedCart {
	key := prewarmKey(accountID, planCode, datacenter, options, coupon, duration)
	prewarmMu.Lock()
	defer prewarmMu.Unlock()

	c, ok := prewarmCarts[key]
	if !ok {
		return nil
	}
	delete(prewarmCarts, key)
	if time.Since(c.CreatedAt) > cartTTL {
		// 过期车不再用,撤销登记让遗留车清理把它从 OVH 删掉
		ovh.ReleaseCart(c.CartID)
		return nil
	}
	return c
}

// PutPrewarmedCart 将组装完成的预装车存入池中
func PutPrewarmedCart(cart *PrewarmedCart) {
	if cart == nil || cart.CartID == "" {
		return
	}
	key := prewarmKey(cart.AccountID, cart.PlanCode, cart.Datacenter, cart.Options, cart.Coupon, cart.Duration)
	prewarmMu.Lock()
	old := prewarmCarts[key]
	prewarmCarts[key] = cart
	prewarmMu.Unlock()
	// 监控预热、队列预热、验价留车三条路径可能为同一个键各装一辆,后放的覆盖先放的。
	// 被覆盖的那辆没人会再用:撤销登记,由遗留车清理从 OVH 删掉,不留僵尸车。
	if old != nil && old.CartID != cart.CartID {
		ovh.ReleaseCart(old.CartID)
	}
}

// HasPrewarmedCart 检查指定配置是否已有就绪的预热车
func HasPrewarmedCart(accountID, planCode, datacenter string, options []string, coupon, duration string) bool {
	key := prewarmKey(accountID, planCode, datacenter, options, coupon, duration)
	prewarmMu.Lock()
	defer prewarmMu.Unlock()
	c, ok := prewarmCarts[key]
	if !ok {
		return false
	}
	if time.Since(c.CreatedAt) > cartTTL {
		delete(prewarmCarts, key)
		ovh.ReleaseCart(c.CartID)
		return false
	}
	return true
}

// HasFreshPrewarmedCart 池里有同键、且装好不超过 maxAge 的车。
// 给"有预热车就跳过验价"用:装车成功只说明装车那一刻配置和价格有效,
// 一辆 80 分钟前装的车说明不了现在还买得到,不能拿它顶替验价。
func HasFreshPrewarmedCart(accountID, planCode, datacenter string, options []string, coupon, duration string, maxAge time.Duration) bool {
	key := prewarmKey(accountID, planCode, datacenter, options, coupon, duration)
	prewarmMu.Lock()
	defer prewarmMu.Unlock()
	c, ok := prewarmCarts[key]
	return ok && time.Since(c.CreatedAt) <= maxAge && time.Since(c.CreatedAt) <= cartTTL
}

// ClearPrewarmPool 清理预热池并释放所有购物车（供测试与重置使用）
func ClearPrewarmPool(state *app.State) {
	prewarmMu.Lock()
	defer prewarmMu.Unlock()
	for _, c := range prewarmCarts {
		if c.CartID != "" && state != nil {
			if client, err := state.OVH.IsolatedClientFor(c.AccountID); err == nil {
				_ = client.Delete("/order/cart/"+c.CartID, nil)
			}
			ovh.ReleaseCart(c.CartID)
		}
	}
	prewarmCarts = map[string]*PrewarmedCart{}
}

// AssemblePrewarmedCart 在后台静默组装一辆完整的预热购物车。
// 组装成功后直接存入预热池，供探测发现有货时以 0ms 装车延迟直接结账。
func AssemblePrewarmedCart(ctx context.Context, state *app.State, accountID, planCode, datacenter string, options []string, coupon, duration string) (*PrewarmedCart, error) {
	selectedDuration, err := orderterm.Normalize(duration)
	if err != nil {
		return nil, err
	}
	client, err := state.OVH.IsolatedClientFor(accountID)
	if err != nil {
		return nil, fmt.Errorf("取 client 失败: %w", err)
	}

	acc, ok := state.FindAccount(accountID)
	if !ok {
		return nil, fmt.Errorf("账户 %s 不存在", accountID)
	}
	subsidiary := orderSubsidiary(state, acc, "prewarm")

	// 1. 创建购物车
	var cartResult map[string]interface{}
	if err := client.PostWithContext(ctx, "/order/cart", ovh.CartCreateBody(subsidiary), &cartResult); err != nil {
		return nil, fmt.Errorf("建预热车失败: %w", err)
	}
	cartID, _ := cartResult["cartId"].(string)
	ovh.TrackCart(cartID)
	if cartID == "" {
		return nil, fmt.Errorf("建预热车未返回 cartId")
	}

	cleanOnFail := true
	defer func() {
		if cleanOnFail && cartID != "" {
			_ = client.Delete("/order/cart/"+cartID, nil)
			ovh.ReleaseCart(cartID)
		}
	}()

	// 2. 并发执行：绑定购物车 (assign) + 添加基础商品 (eco)
	var itemResult map[string]interface{}
	var assignErr error
	var postBaseErr error
	baseDuration, basePricingMode := selectedDuration, "default"

	g2, ctx2 := errgroup.WithContext(ctx)
	g2.Go(func() error {
		if err := client.PostWithContext(ctx2, "/order/cart/"+cartID+"/assign", nil, nil); err != nil {
			assignErr = err
			return err
		}
		return nil
	})
	g2.Go(func() error {
		// 非月付先查 OVH 给这个型号的真实计价;查不到该周期就是买不到,不装车
		if selectedDuration != orderterm.Default {
			d, pm, found := lookupEcoPricing(state, client, cartID, planCode, selectedDuration)
			if !found {
				postBaseErr = fmt.Errorf("OVH 不支持 %s 按 %s 购买", planCode, orderterm.Label(selectedDuration))
				return postBaseErr
			}
			baseDuration, basePricingMode = d, pm
		}
		err := client.PostWithContext(ctx2, "/order/cart/"+cartID+"/eco", map[string]interface{}{
			"planCode":    planCode,
			"pricingMode": basePricingMode,
			"duration":    baseDuration,
			"quantity":    1,
		}, &itemResult)
		if err != nil {
			if d, pm, found := lookupEcoPricing(state, client, cartID, planCode, selectedDuration); found &&
				(d != baseDuration || pm != basePricingMode) {
				baseDuration, basePricingMode = d, pm
				err = client.PostWithContext(ctx2, "/order/cart/"+cartID+"/eco", map[string]interface{}{
					"planCode":    planCode,
					"pricingMode": basePricingMode,
					"duration":    baseDuration,
					"quantity":    1,
				}, &itemResult)
			}
			if err != nil {
				postBaseErr = err
				return err
			}
		}
		return nil
	})

	if err := g2.Wait(); err != nil {
		// 两个协程谁先失败,另一个就会被 errgroup 取消、报 context canceled ——
		// 报给用户的必须是真正的那个原因
		if postBaseErr != nil && !errors.Is(postBaseErr, context.Canceled) {
			return nil, fmt.Errorf("预热加购基础商品失败: %w", postBaseErr)
		}
		if assignErr != nil {
			return nil, fmt.Errorf("绑定预热车失败: %w", assignErr)
		}
		if postBaseErr != nil {
			return nil, fmt.Errorf("预热加购基础商品失败: %w", postBaseErr)
		}
		return nil, err
	}

	itemID, _ := numconv.ToInt64(itemResult["itemId"])
	if itemID == 0 {
		return nil, fmt.Errorf("预热加购未解析出 itemId")
	}

	// 3. 准备配置项与选件
	apiDC := ovh.ConvertDisplayDCToAPIDC(datacenter)
	// 与正式下单同一口径(purchase.go):目录兜底分支用的是账户原始 zone,
	// zone 为空的美区账户会算成 europe/canada,必须按归一化后的子公司重算。
	// 不对齐的话预热车带着错 region,结账报 "not available"(不算失效车),
	// 每轮命中这辆坏车失败一次、又装一辆同样坏的,直到用尽重试。
	region, regionSrc := catalog.ResolveRegion(state, accountID, planCode, apiDC)
	if regionSrc == "fallback" || region == "" {
		if r := catalog.FallbackRegion(apiDC, subsidiary); r != "" {
			region = r
		}
	}

	type kv struct{ label, value string }
	configurations := []kv{
		{"dedicated_datacenter", apiDC},
		{"dedicated_os", "none_64.en"},
	}
	if region != "" {
		configurations = append(configurations, kv{"region", region})
	}

	type addonPayload struct {
		planCode string
		body     map[string]interface{}
	}
	var todo []addonPayload
	if len(options) > 0 {
		filtered := filterHardwareOptions(state, options, true)
		if len(filtered) > 0 {
			availableEcoOpts, err := getOrFetchEcoOptions(ctx, client, cartID, subsidiary, planCode)
			if err != nil {
				return nil, fmt.Errorf("预热获取硬件选项失败: %w", err)
			}
			for _, wanted := range filtered {
				matchedOpt, matchedPC, _ := catalog.MatchEcoOption(availableEcoOpts, wanted)
				if matchedOpt == nil {
					return nil, fmt.Errorf("预热硬件选项 %q 未找到，取消装车", wanted)
				}
				duration, pricingMode, valid := pickPricing(matchedOpt["prices"], baseDuration, basePricingMode)
				if !valid {
					return nil, fmt.Errorf("预热硬件选项 %s 不支持购买时长 %s,取消装车", matchedPC, orderterm.Label(baseDuration))
				}
				todo = append(todo, addonPayload{
					planCode: matchedPC,
					body: map[string]interface{}{
						"itemId": itemID, "planCode": matchedPC,
						"duration": duration, "pricingMode": pricingMode, "quantity": 1,
					},
				})
			}
		}
	}

	// 4. 并发写入所有必需配置与硬件选件
	g3, ctx3 := errgroup.WithContext(ctx)
	for _, cfg := range configurations {
		c := cfg
		g3.Go(func() error {
			return client.PostWithContext(ctx3, fmt.Sprintf("/order/cart/%s/item/%d/configuration", cartID, itemID),
				map[string]interface{}{"label": c.label, "value": c.value}, nil)
		})
	}
	for _, t := range todo {
		opt := t
		g3.Go(func() error {
			return client.PostWithContext(ctx3, fmt.Sprintf("/order/cart/%s/eco/options", cartID), opt.body, nil)
		})
	}

	if err := g3.Wait(); err != nil {
		return nil, fmt.Errorf("并发装车写入失败: %w", err)
	}

	// 5. 应用优惠码（若有）
	if cCode := strings.TrimSpace(coupon); cCode != "" {
		if err := client.PostWithContext(ctx, "/order/cart/"+cartID+"/coupon", map[string]interface{}{"coupon": cCode}, nil); err != nil {
			return nil, fmt.Errorf("预热应用优惠码失败: %w", err)
		}
	}

	cleanOnFail = false
	pre := &PrewarmedCart{
		CartID:     cartID,
		AccountID:  accountID,
		PlanCode:   planCode,
		Datacenter: datacenter,
		Options:    append([]string(nil), options...),
		Coupon:     coupon,
		Duration:   selectedDuration,
		CreatedAt:  time.Now(),
	}
	PutPrewarmedCart(pre)
	state.Logger.Info(fmt.Sprintf("⚡ 成功为 %s @ %s 预装购物车 %s 待命", planCode, datacenter, cartID), "prewarm")
	return pre, nil
}
