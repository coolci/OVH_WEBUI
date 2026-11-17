package purchase

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// 结账时绝不能发 waiveRetractationPeriod。
//
// 这个字段的意思是"放弃 14 天无理由撤回权"(官方定义:order will be processed
// with waiving retractation period)。它在 OVH schema 里是 required:false,
// 不传就是不主动弃权。
//
// 为什么值得写个测试盯着:
//   - 这行曾经写死 true 很久,是从最早那次迁移原样带过来的,没人为它写过理由。
//     同样的事很容易再发生一次 —— 它看起来只是个不起眼的布尔。
//   - 方向不对称:传 true 是**当场且永久**地放弃权利,没有任何接口能拿回来;
//     而真需要弃权还有 POST /me/order/{id}/waiveRetraction 可以事后补。
//   - 改错了不会有任何报错:订单照样下成功,只是用户悄悄少了一项权利,
//     要到他想退款那天才发现。没有测试就没人会发现。
//
// 扫源码而不是构造请求:checkout 在 PurchaseServer 中段,要跑到那里得先 mock
// 掉十来个 OVH 调用。这里要守的是"这个字段不出现在 payload 里"这条约束本身,
// 源码层面看得更直接,也不会因为上游流程重构而失效。
func TestCheckoutNeverWaivesRetraction(t *testing.T) {
	files := map[string]string{
		"独服":  "purchase.go",
		"VPS": "../vps/purchase.go",
	}
	for label, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: 读不到 %s: %v", label, path, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if !strings.Contains(line, "waiveRetractationPeriod") {
				continue
			}
			// 注释里提它是好事(解释为什么不发),代码里赋值就不行
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "//") {
				continue
			}
			t.Errorf("%s (%s:%d) 在代码里出现了 waiveRetractationPeriod：\n  %s\n\n"+
				"这个字段一旦传 true,用户当场且永久失去 14 天无理由撤回权,没有接口能撤销。\n"+
				"schema 里它是 required:false,不传即可。真要弃权请走事后的\n"+
				"POST /me/order/{id}/waiveRetraction —— 那条路是可选的、显式的。",
				label, path, i+1, trimmed)
		}
	}
}

// 自动付款功能已取消，旧任务上的 true 也不能进入独服或 VPS 结账请求。
func TestCheckoutAutomaticPaymentDisabled(t *testing.T) {
	re := regexp.MustCompile(`"autoPayWithPreferredPaymentMethod":\s*(\S+?),`)
	for _, path := range []string{"purchase.go", "../vps/purchase.go"} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, line := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			if m := re.FindStringSubmatch(line); m != nil {
				found = true
				if m[1] != "false" {
					t.Fatalf("%s: automatic payment not disabled: %s", path, line)
				}
			}
		}
		if !found {
			t.Fatalf("%s: checkout payment flag missing", path)
		}
	}
}
