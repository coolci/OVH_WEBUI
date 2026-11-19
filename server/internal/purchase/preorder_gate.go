package purchase

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/types"
)

// 预订任务的"共享探测"闸门。
//
// 开售前,一个预订任务每轮要打约 6 次 OVH(查库存、建车、绑定、加购被拒、删车…),
// 而同一型号同一配置往往按机房 × 数量扇出成好几个任务,每 15 秒一轮一起打,
// 白白消耗账户配额、不停建车删车。实际上只要其中一个任务刚被 OVH 明确告知
// "还没开放",同型号同配置的其它任务在短时间内再试结果必然一样。
//
// 所以:一个任务拿到明确的"未开放"拒绝后关闭闸门 preorderGateTTL,
// 期间同型号同配置的其它任务直接按"等待"返回,不打 OVH。
// 被库存/目录唤醒的任务(开售信号)无视闸门立即重试。
const preorderGateTTL = 10 * time.Second

var preorderGate sync.Map // key → gateMark

// gateMark 最近一次明确"未开放"的时间与探测者。
// 闸门只拦"别的"任务:探测者自己下一轮照常探测(它本来就隔着一个轮询间隔),
// 这样同型号同配置任意时刻只有一个任务在真打 OVH。
type gateMark struct {
	at time.Time
	by string
}

type stockWokenKey struct{}

// WithStockWoken 标记本轮是被开售 / 补货信号唤醒的:预订闸门对它不生效。
func WithStockWoken(ctx context.Context) context.Context {
	return context.WithValue(ctx, stockWokenKey{}, true)
}

func stockWoken(ctx context.Context) bool {
	v, _ := ctx.Value(stockWokenKey{}).(bool)
	return v
}

func preorderGateKey(item *types.QueueItem, duration string) string {
	return strings.Join([]string{item.AccountID, item.PlanCode, fingerprint(item.Options), duration}, "\x00")
}

func preorderGateClosed(key, taskID string, now time.Time) bool {
	v, ok := preorderGate.Load(key)
	if !ok {
		return false
	}
	mark := v.(gateMark)
	if now.Sub(mark.at) >= preorderGateTTL {
		preorderGate.CompareAndDelete(key, v)
		return false
	}
	return mark.by != taskID
}

func closePreorderGate(key, taskID string) {
	if key != "" {
		preorderGate.Store(key, gateMark{at: time.Now(), by: taskID})
	}
}

func openPreorderGate(key string) {
	if key != "" {
		preorderGate.Delete(key)
	}
}

// resetPreorderGate 测试用
func resetPreorderGate() {
	preorderGate.Range(func(k, _ any) bool {
		preorderGate.Delete(k)
		return true
	})
}
