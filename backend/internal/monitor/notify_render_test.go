package monitor

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/types"
)

func renderTestMonitor(t *testing.T) *Monitor {
	t.Helper()
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	state := &app.State{
		DB: database,
		Logger: logger.New(filepath.Join(dir, "logs.json"),
			slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))),
	}
	return New(state)
}

// 把用户真正会收到的那条上架通知打印出来。
//
// 通知的排版是这个工具的门面 —— 有货那一刻用户只看得到它，
// 而以前想看一眼长什么样，只能等一次真实补货。
// 跑法：go test ./internal/monitor/ -run TestRenderAvailabilityAlert -v
func TestRenderAvailabilityAlert(t *testing.T) {
	m := renderTestMonitor(t)
	detected := time.Now().Add(-3 * time.Second).Format(time.RFC3339Nano)

	dcs := []map[string]interface{}{
		{"dc": "waw", "detected_time": detected, "raw_status": "1H-low"},
	}
	cfg := map[string]interface{}{
		"display":       "32G + 2x2TB HDD",
		"memory":        "32GB ECC DDR4-2400",
		"storage":       "2x2TB HDD",
		"cached_price":  "€17.99/月",
		"install_price": "€17.99",
		"options":       []string{"ram-32g-ecc-2400", "softraid-2x2000sa"},
	}

	msg, markup := m.buildAvailabilityAlert("24ska01", dcs, cfg, "KS-5",
		"", "trace-9f2a1c", "cfg-77d0")

	fmt.Println("\n┌─────────── Telegram 上架通知 ───────────")
	for _, line := range splitLines(msg) {
		fmt.Println("│ " + line)
	}
	fmt.Println("├─────────── 按钮 ───────────")
	for _, line := range buttonLines(t, markup) {
		fmt.Println("│ " + line)
	}
	fmt.Println("└────────────────────────────")

	// 关键信息必须在,顺带当回归测试
	for _, must := range []string{"KS-5", "32GB ECC DDR4-2400", "2x2TB HDD", "WAW", "1小时内有货 - 低库存", "€17.99"} {
		if !contains(msg, must) {
			t.Errorf("通知里缺少关键信息 %q", must)
		}
	}
}

// 已经有任务在抢同一个型号时，通知里必须提醒 ——
// 补货常连着来好几条通知，对同一台机器按两次就是两笔真实订单。
func TestAlertWarnsAboutExistingQueue(t *testing.T) {
	m := renderTestMonitor(t)
	m.state.QueueMu.Lock()
	m.state.Queue = []types.QueueItem{
		{ID: "q1", PlanCode: "24sk602", Datacenter: "gra", Status: "running"},
		{ID: "q2", PlanCode: "24sk602", Datacenter: "rbx", Status: "pending"},
		{ID: "q3", PlanCode: "24sk602", Datacenter: "sbg", Status: "completed"}, // 已完成不算
		{ID: "q4", PlanCode: "24sk603", Datacenter: "gra", Status: "running"},   // 别的型号不算
	}
	m.state.QueueMu.Unlock()

	dcs := []map[string]interface{}{{"dc": "gra"}}
	msg, _ := m.buildAvailabilityAlert("24sk602", dcs, nil, "KS-LE-B", "", "", "")

	if !contains(msg, "已经有 2 个任务在抢") {
		t.Fatalf("应提醒已有 2 个进行中的任务，实际通知：\n%s", msg)
	}

	fmt.Println("\n┌────── 已在抢时的提醒 ──────")
	for _, line := range splitLines(msg) {
		fmt.Println("│ " + line)
	}
	fmt.Println("└───────────────────────────")
}

// 没有任务在抢时不该凭空冒出这句提醒
func TestAlertNoQueueWarningWhenIdle(t *testing.T) {
	m := renderTestMonitor(t)
	msg, _ := m.buildAvailabilityAlert("24sk602",
		[]map[string]interface{}{{"dc": "gra"}}, nil, "KS-LE-B", "", "", "")
	if contains(msg, "个任务在抢") {
		t.Fatalf("没有进行中的任务时不该有这句提醒：\n%s", msg)
	}
}

func splitLines(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}

func contains(h, n string) bool {
	return len(n) == 0 || (len(h) >= len(n) && indexOf(h, n) >= 0)
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}

// buttonLines 把 inline_keyboard 里每一行按钮渲染成一行文本。
// 键盘里的按钮是函数内的局部类型，直接断言不了，走 JSON 最稳。
func buttonLines(t *testing.T, markup map[string]interface{}) []string {
	t.Helper()
	raw, err := json.Marshal(markup["inline_keyboard"])
	if err != nil {
		t.Fatalf("marshal keyboard: %v", err)
	}
	var rows [][]struct {
		Text         string `json:"text"`
		CallbackData string `json:"callback_data"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatalf("unmarshal keyboard: %v", err)
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		line := ""
		for _, b := range row {
			line += "[ " + b.Text + " ]  "
		}
		out = append(out, line)
	}
	return out
}

// 多机房:每个机房的可用性可能不一样(waw 是 1H-low、gra 是 72H)。
// 合并成一句「N 个机房有货」会把这个差别抹掉,而它直接决定先抢哪个。
func TestRenderMultiDCAlert(t *testing.T) {
	m := renderTestMonitor(t)
	detected := time.Now().Add(-2 * time.Second).Format(time.RFC3339Nano)
	dcs := []map[string]interface{}{
		{"dc": "waw", "detected_time": detected, "raw_status": "1H-low"},
		{"dc": "gra", "detected_time": detected, "raw_status": "24H"},
		{"dc": "bhs", "detected_time": detected, "raw_status": "720H"},
	}
	cfg := map[string]interface{}{
		"memory":        "32GB ECC DDR4-2400",
		"storage":       "2x2TB HDD",
		"cached_price":  "€17.99/月",
		"install_price": "€17.99",
	}
	msg, _ := m.buildAvailabilityAlert("24ska01", dcs, cfg, "KS-5", "", "", "")

	fmt.Println("\n┌────── 多机房 ──────")
	for _, line := range splitLines(msg) {
		fmt.Println("│ " + line)
	}
	fmt.Println("└───────────────────")

	// 三个机房各自的可用性都要出现,不能被合并成一句
	for _, must := range []string{"1小时内有货 - 低库存", "24小时内有货", "720小时内有货（约30天）"} {
		if !contains(msg, must) {
			t.Errorf("多机房时每个机房的可用性都要列出来，缺少 %q", must)
		}
	}
	if !contains(msg, "3 个机房有货") {
		t.Error("应当有机房总数")
	}
}

// 价格查不到时必须明说。留空会被读成「免费」或「还没加载」，
// 这两种理解都会让用户按下一个不知道要花多少钱的按钮。
func TestRenderAlertSaysWhenPriceMissing(t *testing.T) {
	m := renderTestMonitor(t)
	dcs := []map[string]interface{}{{"dc": "gra", "raw_status": "24H"}}
	msg, _ := m.buildAvailabilityAlert("24ska01", dcs,
		map[string]interface{}{"memory": "32G", "storage": "2x2TB"},
		"KS-5", "询价超时", "", "")
	if !contains(msg, "未获取到") {
		t.Fatalf("价格缺失必须明说，实际：\n%s", msg)
	}
	if !contains(msg, "询价超时") {
		t.Fatalf("失败原因要带上，实际：\n%s", msg)
	}
}

func TestRenderUnavailableAlertGrouped(t *testing.T) {
	m := renderTestMonitor(t)
	unavailDCs := []map[string]interface{}{
		{"dc": "fra"},
		{"dc": "gra"},
		{"dc": "lon"},
		{"dc": "rbx"},
		{"dc": "sbg"},
		{"dc": "waw"},
		{"dc": "bhs"},
	}
	cfg := map[string]interface{}{
		"display": "ram-64g-ecc-2133 + softraid-2x450nvme",
		"memory":  "ram-64g-ecc-2133",
		"storage": "softraid-2x450nvme",
	}

	msg := m.buildUnavailableAlertGrouped("24sk202", unavailDCs, cfg,
		"KS-2 | Intel Xeon-D 1540",
		"281f9a29-ca65-46e5-b419-0d0635b1449e",
		"c79b61b6-0c94-4210-8376-a4815b9c03ea",
	)

	fmt.Println("\n┌────── 下架聚合通知渲染 ──────")
	for _, line := range splitLines(msg) {
		fmt.Println("│ " + line)
	}
	fmt.Println("└─────────────────────────────")

	// 关键信息验证：规格必须人性化，不能包含原生冗长 slug，必须有 7 个机房
	for _, must := range []string{
		"📦 服务器下架通知 · 24sk202",
		"KS-2 | Intel Xeon-D 1540",
		"64G ECC ｜ 2×450G NVMe",
		"已下架机房 (7 个)",
		"FRA", "GRA", "LON", "RBX", "SBG", "WAW", "BHS",
		"281f9a29", "c79b61b6",
	} {
		if !contains(msg, must) {
			t.Errorf("通知中缺少关键信息: %q", must)
		}
	}
	// 绝对不能有未翻译的原始 slug
	if contains(msg, "ram-64g-ecc-2133") || contains(msg, "softraid-2x450nvme") {
		t.Errorf("通知中包含了未翻译的原始硬件 slug:\n%s", msg)
	}
}

// 验证安装费与纯月费的展示逻辑：有安装费时拆开展示纯月费与首月合计，无安装费时只显示单月价格
func TestRenderPriceDisplayVariants(t *testing.T) {
	m := renderTestMonitor(t)
	dcs := []map[string]interface{}{{"dc": "fra", "raw_status": "1H-low"}}

	// Case 1: 带安装费
	cfgWithInstall := map[string]interface{}{
		"cached_price":      "€23.99/月",
		"install_price":     "€35.99",
		"first_month_price": "€59.98",
	}
	msg1, _ := m.buildAvailabilityAlert("24sys01-v1", dcs, cfgWithInstall, "SYS-1", "", "", "")
	if !contains(msg1, "💰 月付价格: €23.99/月") {
		t.Errorf("应显示清晰的纯月付价格标签，实际：\n%s", msg1)
	}
	if !contains(msg1, "💵 安装费用: €35.99（一次性）") {
		t.Errorf("应包含独立的安装费行，实际：\n%s", msg1)
	}
	if !contains(msg1, "🧾 首月合计: €59.98") {
		t.Errorf("应独立换行优雅展示首月合计，实际：\n%s", msg1)
	}

	// Case 2: 无安装费
	cfgNoInstall := map[string]interface{}{
		"cached_price": "€23.99/月",
	}
	msg2, _ := m.buildAvailabilityAlert("24sys01-v1", dcs, cfgNoInstall, "SYS-1", "", "", "")
	if !contains(msg2, "💰 价格: €23.99/月") {
		t.Errorf("无安装费时应显示通用价格标签，实际：\n%s", msg2)
	}
	if contains(msg2, "安装费用") {
		t.Errorf("无安装费时不应出现安装费行，实际：\n%s", msg2)
	}
}

// 验证现代化按钮渲染（多机房排版、全节点入队等）
func TestRenderModernButtons(t *testing.T) {
	m := renderTestMonitor(t)
	dcs := []map[string]interface{}{
		{"dc": "fra", "raw_status": "1H-low"},
		{"dc": "gra", "raw_status": "24H"},
		{"dc": "bhs", "raw_status": "72H"},
	}
	cfg := map[string]interface{}{
		"cached_price": "€23.99/月",
	}
	_, markup := m.buildAvailabilityAlert("24sys01-v1", dcs, cfg, "SYS-1", "", "", "")
	lines := buttonLines(t, markup)

	fmt.Println("\n┌─────────── 现代化多机房按钮渲染 ───────────")
	for _, l := range lines {
		fmt.Println("│ " + l)
	}
	fmt.Println("└──────────────────────────────────────────")

	raw, _ := json.Marshal(markup)
	rawStr := string(raw)
	if !contains(rawStr, "⚡ 抢购 · 🇩🇪 FRA") {
		t.Errorf("缺少 FRA 抢购按钮: %s", rawStr)
	}
	if !contains(rawStr, "⚡ 抢购 · 🇫🇷 GRA") {
		t.Errorf("缺少 GRA 抢购按钮: %s", rawStr)
	}
	if !contains(rawStr, "📥 全节点挂机入队") {
		t.Errorf("缺少全节点挂机入队按钮: %s", rawStr)
	}
}



