package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/types"
)

// VPS 自动下单重新开启后的编辑接口:字段按传了才改;开自动下单必须有账户;
// 数量夹到 [1, MaxOrderQuantity];关掉自动下单时一并关掉自动付款;系统(OS)字段真正生效。
func TestUpdateVPSSubscriptionAutoOrderFields(t *testing.T) {
	st := newTestState(t)
	st.Accounts = []types.OVHAccount{{ID: "eu", Endpoint: "ovh-eu", Zone: "IE"}, {ID: "us", Endpoint: "ovh-us", Zone: "US"}}
	if err := st.AddVPSSubscription(types.VPSSubscription{ID: "v1", PlanCode: "vps-2025-model1", OvhSubsidiary: "IE",
		MonitorLinux: true, NotifyAvailable: true, LastStatus: map[string]string{"eu-west-gra": "available"},
		History: []map[string]interface{}{}, CreatedAt: types.NowISO()}); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.PUT("/vps/:subscription_id", UpdateVPSSubscription(st))
	put := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, "/vps/v1", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}
	live := func() types.VPSSubscription {
		s, ok := st.FindVPSSubscription("v1")
		if !ok {
			t.Fatal("subscription vanished")
		}
		return *s
	}

	if w := put(`{"autoOrder":true}`); w.Code != http.StatusBadRequest {
		t.Fatalf("auto-order without an account must be rejected: %d %s", w.Code, w.Body.String())
	}
	if w := put(`{"autoOrder":true,"autoOrderAccountId":"us"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("a US account cannot order an IE subscription: %d %s", w.Code, w.Body.String())
	}
	if w := put(`{"autoOrder":true,"autoOrderAccountId":"eu","quantity":999,"autoPay":true,"os":" Debian 12 "}`); w.Code != http.StatusOK {
		t.Fatalf("enable: %d %s", w.Code, w.Body.String())
	}
	got := live()
	if !got.AutoOrder || got.AutoOrderAccountID != "eu" || got.Quantity != types.MaxOrderQuantity || !got.AutoPay || got.OS != "Debian 12" {
		t.Fatalf("auto-order settings not applied: %+v", got)
	}
	if got.LastStatus["eu-west-gra"] != "available" {
		t.Fatal("editing order settings must not reset the stock baseline")
	}
	// 只改通知,不传下单字段:自动下单设置原样保留
	if w := put(`{"notifyUnavailable":true}`); w.Code != http.StatusOK || !live().AutoOrder || live().Quantity != types.MaxOrderQuantity {
		t.Fatalf("a partial edit must keep auto-order settings: %d %+v", w.Code, live())
	}
	// 关掉自动下单:自动付款与数量一并归零
	if w := put(`{"autoOrder":false}`); w.Code != http.StatusOK {
		t.Fatalf("disable: %d %s", w.Code, w.Body.String())
	}
	if got := live(); got.AutoOrder || got.AutoPay || got.Quantity != 0 {
		t.Fatalf("auto-pay must follow auto-order off: %+v", got)
	}
	persisted, err := st.DB.ListVPSSubscriptions()
	if err != nil || len(persisted) != 1 || persisted[0].AutoOrder || persisted[0].OS != "Debian 12" {
		t.Fatalf("edits must be persisted: %+v err=%v", persisted, err)
	}
}

func TestClampVPSQuantity(t *testing.T) {
	for in, want := range map[int]int{-3: 1, 0: 1, 1: 1, 2: 2, types.MaxOrderQuantity + 5: types.MaxOrderQuantity} {
		if got := clampVPSQuantity(in); got != want {
			t.Errorf("clampVPSQuantity(%d)=%d want %d", in, got, want)
		}
	}
}
