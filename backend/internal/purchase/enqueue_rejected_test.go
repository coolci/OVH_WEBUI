package purchase

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/types"
)

// 业务规则拒绝(已有同配置任务在抢)不是存储故障:返回 409 + 原因本身,
// 不能再说"任务没能写进数据库,已撤回"。
func TestMonitorDuplicateEnqueueIsReportedAsRejection(t *testing.T) {
	s := quickOrderTestState(t)
	s.Accounts = []types.OVHAccount{{ID: "a", Endpoint: "ovh-eu", Zone: "FI", AppKey: "k", AppSecret: "s", ConsumerKey: "c"}}
	PutPrewarmedCart(&PrewarmedCart{CartID: "c", AccountID: "a", PlanCode: "p", Datacenter: "gra", Options: []string{"x"}, CreatedAt: time.Now()})
	t.Cleanup(func() { ClearPrewarmPool(nil) })
	params := QuickOrderParams{AccountID: "a", PlanCode: "p", Datacenter: "gra", Options: []string{"x"},
		FromMonitor: true, SkipDuplicateCheck: true, Force: true, Quantity: 1}
	if res := EnqueueQuickOrder(s, params); !res.Success {
		t.Fatalf("first enqueue: %+v", res)
	}
	res := EnqueueQuickOrder(s, params)
	if res.Success || res.HTTPStatus != http.StatusConflict || res.Code != "ENQUEUE_REJECTED" {
		t.Fatalf("a duplicate must be a 409 rejection: %+v", res)
	}
	if strings.Contains(res.Error, "数据库") || !strings.Contains(res.Error, "正在进行的抢购任务") {
		t.Fatalf("the message must state the actual reason, not a storage failure: %q", res.Error)
	}
	s.QueueMu.Lock()
	defer s.QueueMu.Unlock()
	if len(s.Queue) != 1 {
		t.Fatalf("still exactly one task expected, got %d", len(s.Queue))
	}
}
