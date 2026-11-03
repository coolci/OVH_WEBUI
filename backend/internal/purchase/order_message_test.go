package purchase

import (
	"fmt"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/types"
)

// 这是整个系统里唯一一条"你花钱了"的消息,措辞必须能单独审。
func TestDumpOrderMessages(t *testing.T) {
	item := &types.QueueItem{
		ID:         "a1b2c3d4-1111-2222-3333",
		PlanCode:   "24ska01",
		Datacenter: "waw",
		Options:    []string{"ram-32g-ecc-2400", "softraid-2x2000sa"},
	}
	url := "https://manager.eu.ovhcloud.com/dedicated/#/billing/order?orderId=108523471"

	fmt.Println("\n════════ 抢购成功（默认：不自动付款）════════")
	fmt.Println(BuildOrderSuccessMessage(item, "108523471", url))

	item.AutoPay = true
	fmt.Println("\n════════ 抢购成功（开了自动付款）════════")
	fmt.Println(BuildOrderSuccessMessage(item, "108523471", url))

	fmt.Println("\n════════ 拿不到控制面板链接时 ════════")
	item.AutoPay = false
	fmt.Println(BuildOrderSuccessMessage(item, "108523471", ""))

	fmt.Println("\n════════ 带阶段耗时指标（常规抢购）════════")
	tl := newTimeline()
	tl.items = []phase{
		{Name: "查库存", Dur: 395 * time.Millisecond},
		{Name: "建购物车", Dur: 564 * time.Millisecond},
		{Name: "绑定并加购", Dur: 1150 * time.Millisecond},
		{Name: "必需配置与选件", Dur: 1070 * time.Millisecond},
		{Name: "下单", Dur: 4450 * time.Millisecond},
	}
	tl.start = time.Now().Add(-7629 * time.Millisecond)
	fmt.Println(BuildOrderSuccessMessage(item, "108523471", url, tl))

	fmt.Println("\n════════ 带阶段耗时指标（预热秒杀）════════")
	tlPre := newTimeline()
	tlPre.items = []phase{
		{Name: "查库存", Dur: 210 * time.Millisecond},
		{Name: "预装车就绪", Dur: 0},
		{Name: "下单", Dur: 1040 * time.Millisecond},
	}
	tlPre.start = time.Now().Add(-1250 * time.Millisecond)
	fmt.Println(BuildOrderSuccessMessage(item, "108523471", url, tlPre))
}
