package purchase

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/numconv"
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
	CreatedAt  time.Time
}

var (
	prewarmMu    sync.Mutex
	prewarmCarts = map[string]*PrewarmedCart{}
	cartTTL      = 90 * time.Minute // OVH 购物车有效期通常为 2 小时，保守设为 90 分钟
)

func prewarmKey(accountID, planCode, datacenter string, options []string, coupon string) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s",
		accountID,
		planCode,
		strings.ToLower(datacenter),
		fingerprint(options),
		strings.TrimSpace(coupon),
	)
}

// TakePrewarmedCart 获取并消费已就绪的预装车。一旦取出，从池中移除。
func TakePrewarmedCart(accountID, planCode, datacenter string, options []string, coupon string) *PrewarmedCart {
	key := prewarmKey(accountID, planCode, datacenter, options, coupon)
	prewarmMu.Lock()
	defer prewarmMu.Unlock()

	c, ok := prewarmCarts[key]
	if !ok {
		return nil
	}
	delete(prewarmCarts, key)
	if time.Since(c.CreatedAt) > cartTTL {
		return nil
	}
	return c
}

// PutPrewarmedCart 将组装完成的预装车存入池中
func PutPrewarmedCart(cart *PrewarmedCart) {
	if cart == nil || cart.CartID == "" {
		return
	}
	key := prewarmKey(cart.AccountID, cart.PlanCode, cart.Datacenter, cart.Options, cart.Coupon)
	prewarmMu.Lock()
	prewarmCarts[key] = cart
	prewarmMu.Unlock()
}

// HasPrewarmedCart 检查指定配置是否已有就绪的预热车
func HasPrewarmedCart(accountID, planCode, datacenter string, options []string, coupon string) bool {
	key := prewarmKey(accountID, planCode, datacenter, options, coupon)
	prewarmMu.Lock()
	defer prewarmMu.Unlock()
	c, ok := prewarmCarts[key]
	if !ok {
		return false
	}
	if time.Since(c.CreatedAt) > cartTTL {
		delete(prewarmCarts, key)
		return false
	}
	return true
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
		}
	}
	prewarmCarts = map[string]*PrewarmedCart{}
}

// AssemblePrewarmedCart 在后台静默组装一辆完整的预热购物车。
// 组装成功后直接存入预热池，供探测发现有货时以 0ms 装车延迟直接结账。
func AssemblePrewarmedCart(ctx context.Context, state *app.State, accountID, planCode, datacenter string, options []string, coupon string) (*PrewarmedCart, error) {
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
	if err := client.PostWithContext(ctx, "/order/cart", map[string]interface{}{
		"ovhSubsidiary": subsidiary,
	}, &cartResult); err != nil {
		return nil, fmt.Errorf("建预热车失败: %w", err)
	}
	cartID, _ := cartResult["cartId"].(string)
	if cartID == "" {
		return nil, fmt.Errorf("建预热车未返回 cartId")
	}

	cleanOnFail := true
	defer func() {
		if cleanOnFail && cartID != "" {
			_ = client.Delete("/order/cart/"+cartID, nil)
		}
	}()

	// 2. 并发执行：绑定购物车 (assign) + 添加基础商品 (eco)
	var itemResult map[string]interface{}
	var assignErr error
	var postBaseErr error
	baseDuration, basePricingMode := "P1M", "default"

	g2, ctx2 := errgroup.WithContext(ctx)
	g2.Go(func() error {
		if err := client.PostWithContext(ctx2, "/order/cart/"+cartID+"/assign", nil, nil); err != nil {
			assignErr = err
			return err
		}
		return nil
	})
	g2.Go(func() error {
		err := client.PostWithContext(ctx2, "/order/cart/"+cartID+"/eco", map[string]interface{}{
			"planCode":    planCode,
			"pricingMode": basePricingMode,
			"duration":    baseDuration,
			"quantity":    1,
		}, &itemResult)
		if err != nil {
			if d, pm, found := lookupEcoPricing(state, client, cartID, planCode); found &&
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
	region, _ := catalog.ResolveRegion(state, accountID, planCode, apiDC)
	if region == "" {
		region = catalog.FallbackRegion(apiDC, subsidiary)
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
				duration, pricingMode := pickPricing(matchedOpt["prices"], baseDuration)
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
		CreatedAt:  time.Now(),
	}
	PutPrewarmedCart(pre)
	state.Logger.Info(fmt.Sprintf("⚡ 成功为 %s @ %s 预装购物车 %s 待命", planCode, datacenter, cartID), "prewarm")
	return pre, nil
}
