package purchase

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// failNotice 把同一批任务的终止通知合并成一条。
//
// 为什么必须合并:一条 /buy 最多扇出 60 个任务(MaxOrderFanout),
// 而跨区 planCode 这类 Fatal 是**每个任务第一轮就判**的 ——
// 不合并的话用户会在几秒内收到 60 条一模一样的消息,
// 手机被刷屏不说,真正要看的那条原因也被淹了。
//
// 合并键是 (planCode, 原因),因为同一批失败的原因必然相同;
// 机房不同不拆开 —— "24sk602 在 6 个机房全失败"是一件事,不是六件。
var failNotice = &failNotifier{seen: map[string]*failGroup{}}

type failGroup struct {
	first    time.Time
	count    int
	notified bool
	planCode string
	reason   string
	fatal    bool
	dcs      []string
}

type failNotifier struct {
	mu   sync.Mutex
	seen map[string]*failGroup
}

// failNoticeWindow 同一个 (planCode, 原因) 在这个窗口内只发一条。
// 30 秒足够覆盖一批扇出任务的首轮判定(它们几乎同时跑),
// 又短到不会把"半分钟后又一批真的失败了"给吞掉。
const failNoticeWindow = 30 * time.Second

// take 判断这条终止要不要发通知。返回 (消息, 是否发送)。
// 窗口内第一条立刻发并带上"还会有更多"的说明;后续的只累加计数,不再发。
func (f *failNotifier) take(item *types.QueueItem, reason string, fatal bool) (string, bool) {
	key := item.PlanCode + "|" + reason
	now := time.Now()

	f.mu.Lock()
	defer f.mu.Unlock()
	// 顺手清过期的,不引定时器
	for k, g := range f.seen {
		if now.Sub(g.first) > failNoticeWindow {
			delete(f.seen, k)
		}
	}
	g, ok := f.seen[key]
	if !ok {
		g = &failGroup{first: now, planCode: item.PlanCode, reason: reason, fatal: fatal}
		f.seen[key] = g
	}
	g.count++
	if item.Datacenter != "" && len(g.dcs) < 8 {
		dup := false
		for _, d := range g.dcs {
			if d == item.Datacenter {
				dup = true
				break
			}
		}
		if !dup {
			g.dcs = append(g.dcs, item.Datacenter)
		}
	}
	if g.notified {
		return "", false
	}
	g.notified = true
	return buildTaskFailedMessage(item, reason, fatal), true
}

// buildTaskFailedMessage 任务终止时发给用户的通知。
//
// 要回答三件事:哪一单、为什么停、我现在能做什么。
// 尤其是"能做什么"—— Fatal 和用尽重试的下一步完全不同:
// 前者改了才有用(换本区 planCode / 重新配账户),后者可以直接重开。
func buildTaskFailedMessage(item *types.QueueItem, reason string, fatal bool) string {
	var b strings.Builder
	if fatal {
		b.WriteString(fmt.Sprintf("🛑 抢购任务已停止（重试也不会变）· %s\n", item.PlanCode))
	} else {
		b.WriteString(fmt.Sprintf("⚠️ 抢购任务已停止（连续下单失败）· %s\n", item.PlanCode))
	}
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("型号：" + item.PlanCode + "\n")
	if item.Datacenter != "" {
		b.WriteString("机房：" + telegram.DisplayDCFull(item.Datacenter) + " (" + strings.ToUpper(item.Datacenter) + ")\n")
	}
	if len(item.Options) > 0 {
		b.WriteString("⚙️ 硬件规格：" + telegram.HumanizeOptionCodes(item.Options) + "\n")
	}
	b.WriteString("原因：" + reason + "\n")
	b.WriteString(telegram.CardDivider + "\n")
	if fatal {
		b.WriteString("ℹ️ 这类失败换个时间点重试结果一样，需要先改掉原因。\n" +
			"常见的是型号和账户不在同一个区 —— 三个大区的目录互不相通，\n" +
			"同一台机器在不同区是不同的型号代码。发 /accounts 看当前账户。\n")
	} else {
		b.WriteString("ℹ️ 库存可能已经被抢完。想接着抢就重新下一单。\n")
	}
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString("⚡ 发送 /tasks 查看任务队列（历史里有完整报错）")
	return b.String()
}

type queuePrewarmCoord struct {
	mu           sync.Mutex
	inFlight     map[string]bool
	lastAttempt  map[string]time.Time
	failedCounts map[string]int
}

var globalQueuePrewarmCoord = &queuePrewarmCoord{
	inFlight:     map[string]bool{},
	lastAttempt:  map[string]time.Time{},
	failedCounts: map[string]int{},
}

// ResetQueuePrewarmCoord 重置队列预热协调器（供测试使用）
func ResetQueuePrewarmCoord() {
	globalQueuePrewarmCoord.mu.Lock()
	defer globalQueuePrewarmCoord.mu.Unlock()
	globalQueuePrewarmCoord.inFlight = map[string]bool{}
	globalQueuePrewarmCoord.lastAttempt = map[string]time.Time{}
	globalQueuePrewarmCoord.failedCounts = map[string]int{}
}

// TriggerPrewarmForQueueItem 为处于运行中（待抢购）的队列任务触发后台预热装车
func TriggerPrewarmForQueueItem(ctx context.Context, state *app.State, item *types.QueueItem) {
	if item == nil || item.Status != "running" || item.PlanCode == "" || item.Datacenter == "" {
		return
	}
	// 预订不预热:下单时要逐项核对购物车,而且开售前的预热车装不起来,只会白刷 OVH
	if item.Preorder {
		return
	}
	if state == nil || state.OVH == nil {
		return
	}
	acc, ok := state.FindAccount(item.AccountID)
	if !ok {
		return
	}
	accountID := acc.ID

	// 没指定配件:下单时会从有货的 FQN 推断配件,预热也按同样口径推断,否则永远对不上号
	if len(item.Options) == 0 {
		itemCopy := *item
		sets, ready := inferredPrewarmOptions(state, accountID, item, func() {
			// 库存列表一拉回来就立刻按推断出的配件预热,不等下一轮
			TriggerPrewarmForQueueItem(ctx, state, &itemCopy)
		})
		if !ready {
			return
		}
		for _, opts := range sets {
			prewarmQueueItemWith(state, item, accountID, opts)
		}
		return
	}
	prewarmQueueItemWith(state, item, accountID, item.Options)
}

// prewarmQueueItemWith 按给定配件为队列任务预热一辆车(已有 / 正在装 / 失败冷却中则跳过)
func prewarmQueueItemWith(state *app.State, item *types.QueueItem, accountID string, options []string) {
	key := prewarmKey(accountID, item.PlanCode, item.Datacenter, options, item.Coupon, item.Duration)

	globalQueuePrewarmCoord.mu.Lock()
	if globalQueuePrewarmCoord.inFlight[key] {
		globalQueuePrewarmCoord.mu.Unlock()
		return
	}
	if last, ok := globalQueuePrewarmCoord.lastAttempt[key]; ok {
		failCount := globalQueuePrewarmCoord.failedCounts[key]
		if failCount > 0 {
			shift := failCount
			if shift > 5 {
				shift = 5
			}
			cooldown := time.Duration(1<<shift) * 30 * time.Second
			if time.Since(last) < cooldown {
				globalQueuePrewarmCoord.mu.Unlock()
				return
			}
		}
	}
	if HasPrewarmedCart(accountID, item.PlanCode, item.Datacenter, options, item.Coupon, item.Duration) {
		globalQueuePrewarmCoord.mu.Unlock()
		return
	}

	globalQueuePrewarmCoord.inFlight[key] = true
	globalQueuePrewarmCoord.lastAttempt[key] = time.Now()
	globalQueuePrewarmCoord.mu.Unlock()

	go func() {
		defer func() {
			globalQueuePrewarmCoord.mu.Lock()
			delete(globalQueuePrewarmCoord.inFlight, key)
			globalQueuePrewarmCoord.mu.Unlock()
		}()

		state.Logger.Info(fmt.Sprintf("[queue/prewarm] 开始为队列任务 %s (%s@%s) 预热购物车...",
			item.ID, item.PlanCode, item.Datacenter), "purchase")

		pwCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()

		cart, err := AssemblePrewarmedCart(pwCtx, state, accountID, item.PlanCode, item.Datacenter, options, item.Coupon, item.Duration)
		if err != nil {
			globalQueuePrewarmCoord.mu.Lock()
			globalQueuePrewarmCoord.failedCounts[key]++
			globalQueuePrewarmCoord.mu.Unlock()
			state.Logger.Warn(fmt.Sprintf("[queue/prewarm] 队列任务 %s 预热装车失败: %v", item.ID, err), "purchase")
			return
		}

		globalQueuePrewarmCoord.mu.Lock()
		globalQueuePrewarmCoord.failedCounts[key] = 0
		globalQueuePrewarmCoord.mu.Unlock()

		state.Logger.Info(fmt.Sprintf("[queue/prewarm] ✅ 队列任务预热装车成功就绪！CartID: %s (%s@%s)",
			cart.CartID, item.PlanCode, item.Datacenter), "purchase")
	}()
}

func ensureQueuePrewarmedCarts(ctx context.Context, state *app.State) {
	if state == nil {
		return
	}
	state.QueueMu.Lock()
	runningItems := make([]types.QueueItem, 0, len(state.Queue))
	for _, it := range state.Queue {
		if it.Status == "running" && it.PlanCode != "" && it.Datacenter != "" {
			runningItems = append(runningItems, it)
		}
	}
	state.QueueMu.Unlock()

	for i := range runningItems {
		TriggerPrewarmForQueueItem(ctx, state, &runningItems[i])
	}
}
