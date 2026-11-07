package vps

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/types"
)

// 上游 #11 的四处修复(短机房名 / vps_os 兜底 / 必选附加项 / schema 登记)。
// 本地 VPS 自动下单仍按用户决定保持下线,这里直接驱动内部下单链路,
// 确保将来重新开启时请求参数是 OVH 认的。所有请求都由本地 fixture 应答,不会真下单。

func TestPickMandatoryAddons(t *testing.T) {
	var opts []vpsOption
	if err := json.Unmarshal([]byte(`[
	 {"family":"automatedBackup","mandatory":true,"planCode":"backup-7","prices":[
	  {"duration":"P1M","pricingMode":"default","capacities":["renew"],"price":{"value":1.1}}]},
	 {"family":"automatedBackup","mandatory":true,"planCode":"backup-1","prices":[
	  {"duration":"P0D","pricingMode":"default","capacities":["installation"],"price":{"value":0}},
	  {"duration":"P1M","pricingMode":"default","capacities":["renew"],"price":{"value":0.35}}]},
	 {"family":"storage","mandatory":true,"planCode":"storage-remote","prices":[
	  {"duration":"P1M","pricingMode":"default","capacities":["renew"],"price":{"value":0}}]},
	 {"family":"os","mandatory":true,"planCode":"option-linux","prices":[
	  {"duration":"P1M","pricingMode":"default","capacities":["renew"],"price":{"value":0}}]},
	 {"family":"snapshot","mandatory":false,"planCode":"snap","prices":[
	  {"duration":"P1M","pricingMode":"default","capacities":["renew"],"price":{"value":0.3}}]}
	]`), &opts); err != nil {
		t.Fatal(err)
	}
	picks, missing := pickMandatoryAddons(opts, "P1M", "default")
	if len(missing) != 0 {
		t.Fatalf("不该有缺失家族: %v", missing)
	}
	got := map[string]string{}
	for _, p := range picks {
		got[p.family] = p.planCode
	}
	if len(got) != 2 || got["automatedBackup"] != "backup-1" || got["storage"] != "storage-remote" {
		t.Errorf("挑选结果不对: %v", got)
	}
	// 计价方式对不上 → 报缺失,而不是硬塞一个会被 checkout 拒的
	if _, missing = pickMandatoryAddons(opts, "P1Y", "upfront12"); len(missing) != 2 {
		t.Errorf("P1Y 下两个必选家族都该缺失,实际 %v", missing)
	}
	// 老型号没有任何选项 → 无必选项,照常继续
	if picks, missing = pickMandatoryAddons(nil, "P1M", "default"); len(picks) != 0 || len(missing) != 0 {
		t.Errorf("没有选项时应为空: picks=%v missing=%v", picks, missing)
	}
}

func TestRestockOrderDatacenterPrefersShortName(t *testing.T) {
	cases := []struct {
		dc   map[string]interface{}
		want string
	}{
		{map[string]interface{}{"name": "DE", "code": "eu-west-lim"}, "DE"},
		{map[string]interface{}{"name": "UK", "code": "eu-west-eri"}, "UK"},
		{map[string]interface{}{"name": " ", "code": "GRA"}, "GRA"},
		{map[string]interface{}{"code": "BHS"}, "BHS"},
		{map[string]interface{}{}, ""},
	}
	for _, c := range cases {
		if got := restockOrderDatacenter(c.dc); got != c.want {
			t.Errorf("restockOrderDatacenter(%v)=%q want %q", c.dc, got, c.want)
		}
	}
}

type recordedReq struct {
	method, path string
	body         map[string]interface{}
}

// 完整走一遍内部下单链路:2027 系列型号,购物车要求 vps_datacenter + vps_os(非必填),
// options 里有两个必选家族。断言:机房交短名、没选系统时兜底 Debian 12、
// 两个必选附加项按主商品计价在 checkout 之前加入,os 家族不手动加。
func TestPurchaseVPSInternalSendsAcceptedConfiguration(t *testing.T) {
	var mu sync.Mutex
	var reqs []recordedReq
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := map[string]interface{}{}
		if raw, _ := io.ReadAll(r.Body); len(raw) > 0 {
			_ = json.Unmarshal(raw, &body)
		}
		mu.Lock()
		reqs = append(reqs, recordedReq{r.Method, r.URL.Path, body})
		mu.Unlock()
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			fmt.Fprint(w, `{"cartId":"vps-cart"}`)
		case r.URL.Path == "/order/cart/vps-cart/assign":
			fmt.Fprint(w, `null`)
		case r.Method == "GET" && r.URL.Path == "/order/cart/vps-cart/vps":
			fmt.Fprint(w, `[{"planCode":"vps-2027-model1","prices":[{"duration":"P1M","pricingMode":"default","capacities":["renew"]}]}]`)
		case r.Method == "POST" && r.URL.Path == "/order/cart/vps-cart/vps":
			fmt.Fprint(w, `{"itemId":42}`)
		case r.URL.Path == "/order/cart/vps-cart/item/42/requiredConfiguration":
			fmt.Fprint(w, `[{"label":"vps_datacenter","required":true,"allowedValues":["DE","GRA","UK"]},
				{"label":"vps_os","required":false,"allowedValues":["Ubuntu 24.04","Debian 12","Windows Server 2022"]}]`)
		case r.URL.Path == "/order/cart/vps-cart/item/42/configuration":
			fmt.Fprint(w, `{}`)
		case r.Method == "GET" && r.URL.Path == "/order/cart/vps-cart/vps/options":
			if r.URL.Query().Get("planCode") != "vps-2027-model1" {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(w, `{"message":"planCode missing"}`)
				return
			}
			fmt.Fprint(w, `[
			 {"family":"automatedBackup","mandatory":true,"planCode":"backup-1","prices":[{"duration":"P1M","pricingMode":"default","capacities":["renew"],"price":{"value":0.35}}]},
			 {"family":"storage","mandatory":true,"planCode":"storage-remote","prices":[{"duration":"P1M","pricingMode":"default","capacities":["renew"],"price":{"value":0}}]},
			 {"family":"os","mandatory":true,"planCode":"option-linux","prices":[{"duration":"P1M","pricingMode":"default","capacities":["renew"],"price":{"value":0}}]}]`)
		case r.Method == "POST" && r.URL.Path == "/order/cart/vps-cart/vps/options":
			fmt.Fprint(w, `{}`)
		case r.URL.Path == "/order/cart/vps-cart/checkout":
			fmt.Fprint(w, `{"orderId":9001,"url":"https://example.invalid/order/9001"}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"message":"unexpected %s %s"}`, r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = server.URL
	t.Cleanup(func() { ovhsdk.Endpoints["ovh-eu"] = orig })

	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	state := app.NewState(storage.Paths{LogsDir: filepath.Join(dir, "logs"), DataDir: dir}, config.New(database),
		logger.New(filepath.Join(dir, "logs.json"), nil), database)
	state.Accounts = []types.OVHAccount{{ID: "acc-vps", Name: "A", Endpoint: "ovh-eu", Zone: "IE",
		AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}}

	sub := types.VPSSubscription{ID: "s1", PlanCode: "vps-2027-model1", OvhSubsidiary: "IE",
		AutoOrder: true, AutoOrderAccountID: "acc-vps", Quantity: 1}
	// 可用性接口给的机房:短名 DE,长码 eu-west-lim
	dcCode := restockOrderDatacenter(map[string]interface{}{"name": "DE", "code": "eu-west-lim"})
	out := purchaseVPSInternal(state, sub, dcCode)
	if !out.Success || out.OrderID != "9001" {
		t.Fatalf("下单应成功: %+v", out)
	}

	mu.Lock()
	defer mu.Unlock()
	configs := map[string]string{}
	var addons []string
	checkoutIdx, lastAddonIdx := -1, -1
	for i, r := range reqs {
		switch {
		case r.method == "POST" && strings.HasSuffix(r.path, "/item/42/configuration"):
			configs[fmt.Sprint(r.body["label"])] = fmt.Sprint(r.body["value"])
		case r.method == "POST" && r.path == "/order/cart/vps-cart/vps/options":
			addons = append(addons, fmt.Sprint(r.body["planCode"]))
			if r.body["duration"] != "P1M" || r.body["pricingMode"] != "default" || fmt.Sprint(r.body["itemId"]) != "42" {
				t.Errorf("附加项计价/归属不对: %v", r.body)
			}
			lastAddonIdx = i
		case strings.HasSuffix(r.path, "/checkout"):
			checkoutIdx = i
		}
	}
	if configs["vps_datacenter"] != "DE" {
		t.Errorf("vps_datacenter 应提交短名 DE,实际 %q", configs["vps_datacenter"])
	}
	if configs["vps_os"] != "Debian 12" {
		t.Errorf("未选系统应兜底 Debian 12,实际 %q", configs["vps_os"])
	}
	if strings.Join(addons, ",") != "backup-1,storage-remote" {
		t.Errorf("必选附加项应为 backup-1,storage-remote(不含 os 家族),实际 %v", addons)
	}
	if checkoutIdx < 0 || lastAddonIdx > checkoutIdx {
		t.Errorf("附加项必须在 checkout 之前加入: addon=%d checkout=%d", lastAddonIdx, checkoutIdx)
	}
}

// 必选家族在主商品计价下没有可选值时,应确定性失败且不结账、并清理购物车
func TestPurchaseVPSInternalStopsWhenMandatoryAddonUnavailable(t *testing.T) {
	var checkouts, deletes int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/auth/time":
			fmt.Fprint(w, time.Now().Unix())
		case r.Method == "POST" && r.URL.Path == "/order/cart":
			fmt.Fprint(w, `{"cartId":"c2"}`)
		case r.Method == "DELETE" && r.URL.Path == "/order/cart/c2":
			deletes++
			fmt.Fprint(w, `null`)
		case r.Method == "GET" && r.URL.Path == "/order/cart/c2/vps":
			fmt.Fprint(w, `[{"planCode":"vps-x","prices":[{"duration":"P1M","pricingMode":"default","capacities":["renew"]}]}]`)
		case r.Method == "POST" && r.URL.Path == "/order/cart/c2/vps":
			fmt.Fprint(w, `{"itemId":7}`)
		case r.URL.Path == "/order/cart/c2/item/7/requiredConfiguration":
			fmt.Fprint(w, `[{"label":"vps_datacenter","required":true}]`)
		case r.Method == "GET" && r.URL.Path == "/order/cart/c2/vps/options":
			fmt.Fprint(w, `[{"family":"automatedBackup","mandatory":true,"planCode":"backup-1","prices":[{"duration":"P12M","pricingMode":"upfront12","capacities":["renew"],"price":{"value":4}}]}]`)
		case strings.HasSuffix(r.URL.Path, "/checkout"):
			checkouts++
			fmt.Fprint(w, `{"orderId":1}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	t.Cleanup(server.Close)
	orig := ovhsdk.Endpoints["ovh-eu"]
	ovhsdk.Endpoints["ovh-eu"] = server.URL
	t.Cleanup(func() { ovhsdk.Endpoints["ovh-eu"] = orig })

	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	state := app.NewState(storage.Paths{LogsDir: filepath.Join(dir, "logs"), DataDir: dir}, config.New(database),
		logger.New(filepath.Join(dir, "logs.json"), nil), database)
	state.Accounts = []types.OVHAccount{{ID: "acc", Name: "A", Endpoint: "ovh-eu", Zone: "IE",
		AppKey: "k", AppSecret: "s", ConsumerKey: "c", IsDefault: true}}

	out := purchaseVPSInternal(state, types.VPSSubscription{ID: "s", PlanCode: "vps-x", OvhSubsidiary: "IE",
		AutoOrderAccountID: "acc", Quantity: 1}, "GRA")
	if out.Success || !out.Fatal || !strings.Contains(out.Reason, "automatedBackup") {
		t.Fatalf("应确定性失败并点名缺失家族: %+v", out)
	}
	mu.Lock()
	defer mu.Unlock()
	if checkouts != 0 {
		t.Errorf("缺必选附加项时不该结账,checkout 调用了 %d 次", checkouts)
	}
	if deletes != 1 {
		t.Errorf("失败后应清理购物车,DELETE 调用了 %d 次", deletes)
	}
}
