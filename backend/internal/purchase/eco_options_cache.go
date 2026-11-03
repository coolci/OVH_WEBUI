package purchase

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"
)

type ecoOptionEntry struct {
	options  []map[string]interface{}
	cachedAt time.Time
}

var (
	ecoOptionsCacheMu sync.RWMutex
	ecoOptionsCache   = map[string]ecoOptionEntry{} // key: subsidiary + ":" + planCode
	ecoOptionsTTL     = 30 * time.Minute
)

// getOrFetchEcoOptions 获取指定 subsidiary 和 planCode 的可用硬件选项。
// 优先读内存缓存，命中即 0ms 返回，避免在抢购关键路径上发起高耗时的 GET /eco/options。
func getOrFetchEcoOptions(ctx context.Context, client *ovhsdk.Client, cartID, subsidiary, planCode string) ([]map[string]interface{}, error) {
	cacheKey := subsidiary + ":" + planCode

	ecoOptionsCacheMu.RLock()
	entry, ok := ecoOptionsCache[cacheKey]
	ecoOptionsCacheMu.RUnlock()

	if ok && time.Since(entry.cachedAt) < ecoOptionsTTL && len(entry.options) > 0 {
		return entry.options, nil
	}

	var availableEcoOpts []map[string]interface{}
	q := url.Values{}
	q.Set("planCode", planCode)
	if err := client.GetWithContext(ctx, fmt.Sprintf("/order/cart/%s/eco/options?%s", cartID, q.Encode()), &availableEcoOpts); err != nil {
		return nil, err
	}

	if len(availableEcoOpts) > 0 {
		ecoOptionsCacheMu.Lock()
		ecoOptionsCache[cacheKey] = ecoOptionEntry{
			options:  availableEcoOpts,
			cachedAt: time.Now(),
		}
		ecoOptionsCacheMu.Unlock()
	}

	return availableEcoOpts, nil
}

// resetEcoOptionsCache 清空选项缓存（供测试与重置使用）
func resetEcoOptionsCache() {
	ecoOptionsCacheMu.Lock()
	ecoOptionsCache = map[string]ecoOptionEntry{}
	ecoOptionsCacheMu.Unlock()
}

