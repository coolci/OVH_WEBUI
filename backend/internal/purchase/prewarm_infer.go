package purchase

import (
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

// 没指定配件的队列任务的预热。
//
// 这类任务下单时,配件是从"当时在该机房有货的那条 FQN"推断的(optionsFromFQN)。
// 以前预热按"空配件"装车、按空配件登记,和下单时推断出的配件永远对不上,
// 每次都从零装车。现在预热也按同一口径推断:取该型号在该机房登记过的 FQN,
// 按 OVH 返回的顺序取前几条(下单时也是取第一条有货的),各热一辆。
//
// FQN 列表很少变化,按 (账户, 型号) 缓存 10 分钟;没缓存时后台去拉,本轮先跳过。
// 队列循环每秒都会调到这里,不能每次都打 OVH。

const (
	fqnSnapshotTTL     = 10 * time.Minute
	maxInferredPrewarm = 2 // 每个任务最多按几种配置组合预热(同一机房往往只有一两种)
)

type fqnSnapshot struct {
	at   time.Time
	byDC map[string][]string // OVH 机房代码 → 列出该机房的 FQN(保持接口返回顺序)
}

var (
	fqnSnapMu    sync.Mutex
	fqnSnaps     = map[string]*fqnSnapshot{}
	fqnSnapFetch = map[string]bool{}
)

// resetFQNSnapshots 清空缓存(测试用)
func resetFQNSnapshots() {
	fqnSnapMu.Lock()
	defer fqnSnapMu.Unlock()
	fqnSnaps = map[string]*fqnSnapshot{}
	fqnSnapFetch = map[string]bool{}
}

// inferredPrewarmOptions 给没指定配件的任务算出要预热的配件组合(最多 maxInferredPrewarm 组)。
// ok=false 表示 FQN 列表还没就绪:已在后台拉取,拉完后调用 onReady(由本次调用发起拉取时)。
// FQN 只有型号、没有配件段时,下单用的就是空配件,这里也给出空组合,口径一致。
// 该机房一条 FQN 都没登记时不预热:猜不出会是哪种组合,空配件车多半用不上。
func inferredPrewarmOptions(state *app.State, accountID string, item *types.QueueItem, onReady func()) ([][]string, bool) {
	key := accountID + "|" + item.PlanCode
	fqnSnapMu.Lock()
	snap := fqnSnaps[key]
	fresh := snap != nil && time.Since(snap.at) < fqnSnapshotTTL
	if !fresh && !fqnSnapFetch[key] {
		fqnSnapFetch[key] = true
		go fetchFQNSnapshot(state, accountID, item.PlanCode, key, onReady)
	}
	fqnSnapMu.Unlock()
	if snap == nil {
		return nil, false
	}

	apiDC := ovh.ConvertDisplayDCToAPIDC(item.Datacenter)
	var sets [][]string
	seen := map[string]bool{}
	for _, fqn := range snap.byDC[apiDC] {
		opts := optionsFromFQN(fqn, item.PlanCode)
		k := strings.Join(opts, ".")
		if seen[k] {
			continue
		}
		seen[k] = true
		if opts == nil {
			opts = []string{}
		}
		sets = append(sets, opts)
		if len(sets) >= maxInferredPrewarm {
			break
		}
	}
	return sets, true
}

func fetchFQNSnapshot(state *app.State, accountID, planCode, key string, onReady func()) {
	stored := false
	defer func() {
		fqnSnapMu.Lock()
		delete(fqnSnapFetch, key)
		fqnSnapMu.Unlock()
		if stored && onReady != nil {
			onReady()
		}
	}()
	if state == nil || state.OVH == nil {
		return
	}
	client, err := state.OVH.IsolatedClientFor(accountID)
	if err != nil {
		return
	}
	var availabilities []map[string]interface{}
	q := url.Values{}
	q.Set("planCode", planCode)
	if err := client.Get("/dedicated/server/datacenter/availabilities?"+q.Encode(), &availabilities); err != nil {
		state.Logger.Debug("预热推断配件:拉库存列表失败 "+planCode+": "+err.Error(), "purchase")
		// 失败也记一笔(沿用旧列表),30 秒后再试 —— 否则队列每秒一轮,会每秒打一次 OVH
		fqnSnapMu.Lock()
		prev := fqnSnaps[key]
		byDC := map[string][]string{}
		if prev != nil {
			byDC = prev.byDC
		}
		fqnSnaps[key] = &fqnSnapshot{at: time.Now().Add(-fqnSnapshotTTL + 30*time.Second), byDC: byDC}
		fqnSnapMu.Unlock()
		return
	}
	byDC := map[string][]string{}
	for _, av := range availabilities {
		fqn, _ := av["fqn"].(string)
		// 与下单一致:只认这个型号自己的 FQN(型号本身,或型号后接配件段)
		if fqn != planCode && !strings.HasPrefix(fqn, planCode+".") {
			continue
		}
		dcs, _ := av["datacenters"].([]interface{})
		for _, raw := range dcs {
			dc, _ := raw.(map[string]interface{})
			name, _ := dc["datacenter"].(string)
			if name == "" {
				continue
			}
			list := byDC[name]
			dup := false
			for _, f := range list {
				if f == fqn {
					dup = true
					break
				}
			}
			if !dup {
				byDC[name] = append(list, fqn)
			}
		}
	}
	fqnSnapMu.Lock()
	fqnSnaps[key] = &fqnSnapshot{at: time.Now(), byDC: byDC}
	fqnSnapMu.Unlock()
	stored = true
}
