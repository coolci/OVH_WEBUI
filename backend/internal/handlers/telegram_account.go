package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// resolvedAccount 一次账户解析的结果。
type resolvedAccount struct {
	Account types.OVHAccount
	Region  string
	// Confident 是不是真的按 planCode 算出来的。
	// false = 退回了当前/默认账户，界面上会提示出来。
	Confident bool
	// Reason 一个账户都没有时的说明。
	Reason string
	// Ambiguous 同一个大区里有多个账户，需要用户挑选。
	Ambiguous []types.OVHAccount
}

// resolveOrderAccount 决定这个 planCode 该落到哪个账户。
// 原则：能算就算，算不出来也绝不拒绝。
func resolveOrderAccount(state *app.State, mon *monitor.Monitor, planCode string) resolvedAccount {
	all := listAccounts(state)
	if len(all) == 0 {
		return resolvedAccount{Reason: "还没有配置 OVH 账户。请到控制台「设置 → OVH 账户」添加。"}
	}
	cur, _ := telegram.ActiveAccount(state)
	fallback := resolvedAccount{Account: cur, Region: strings.ToUpper(strings.TrimSpace(cur.Zone))}

	if mon == nil {
		return fallback
	}
	accID, region, _ := mon.PlanAccountFast(planCode, cur.ID)
	if accID == "" {
		return fallback
	}
	acc, ok := state.FindAccount(accID)
	if !ok {
		return fallback
	}

	if inRegion := mon.AccountsInRegion(region); len(inRegion) > 1 && cur.ID != acc.ID {
		return resolvedAccount{Account: acc, Region: region, Confident: true, Ambiguous: inRegion}
	}
	return resolvedAccount{Account: acc, Region: region, Confident: true}
}

// explainAccountChoice 一句话说明这单为什么落在这个账户上。
func explainAccountChoice(r resolvedAccount, planCode string) string {
	if r.Account.ID == "" {
		return ""
	}
	if r.Confident {
		return fmt.Sprintf("账户：%s\n（%s 在该账户所属大区目录内，已自动选定）",
			telegram.AccountLabel(r.Account), planCode)
	}
	return fmt.Sprintf("账户：%s\n"+
		"⚠️ 没能确认 %s 属于哪个区，用的是当前活跃账户。\n"+
		"   OVH 各大区站点互不相通，用错区下单将无法下单成功。\n"+
		"   如需换账户请发送 /accounts。",
		telegram.AccountLabel(r.Account), planCode)
}

// accountShortName 账户的短名，用在 `@xxx` 里（按子公司代码，如 us, ie, fr, ca 等）。
func accountShortName(a types.OVHAccount) string {
	return strings.ToLower(strings.TrimSpace(a.Zone))
}

// resolveAccountRef 把命令里的 `@xxx` 解析成具体账户。
//
//	"all"    → 所有能买这个 planCode 的账户（各下一单）
//	"<n>"    → /accounts 列表里的第 n 个
//	"<zone>" → 该子公司的账户；有多个时要求用序号
func resolveAccountRef(state *app.State, mon *monitor.Monitor, ref, planCode string) ([]types.OVHAccount, string) {
	all := listAccounts(state)
	if len(all) == 0 {
		return nil, "还没有配置 OVH 账户。"
	}
	ref = strings.ToLower(strings.TrimSpace(ref))

	if ref == "all" {
		if mon != nil {
			if buyable := mon.AccountsForPlan(planCode); len(buyable) > 0 {
				return buyable, ""
			}
		}
		return all, ""
	}

	// @1 / @2：按 /accounts 的显示顺序
	if n, err := strconv.Atoi(ref); err == nil {
		if n < 1 || n > len(all) {
			return nil, fmt.Sprintf("没有第 %d 个账户（一共 %d 个）。发 /accounts 查看序号。", n, len(all))
		}
		return []types.OVHAccount{all[n-1]}, ""
	}

	// @us / @ie：按子公司
	matched := []types.OVHAccount{}
	for _, a := range all {
		if accountShortName(a) == ref {
			matched = append(matched, a)
		}
	}
	switch len(matched) {
	case 1:
		return matched, ""
	case 0:
		names := make([]string, 0, len(all))
		for i, a := range all {
			names = append(names, fmt.Sprintf("@%d(%s)", i+1, accountShortName(a)))
		}
		return nil, fmt.Sprintf("没有叫 @%s 的账户。可用的：%s", ref, strings.Join(names, " "))
	default:
		idx := []string{}
		for i, a := range all {
			if accountShortName(a) == ref {
				idx = append(idx, fmt.Sprintf("@%d(%s)", i+1, a.Name))
			}
		}
		return nil, fmt.Sprintf("@%s 对应 %d 个账户，请用序号明确指定：%s",
			ref, len(matched), strings.Join(idx, " "))
	}
}

// runOrder 对一组账户依次下单，汇总回复。
func runOrder(state *app.State, o *telegram.OrderInfo, accs []types.OVHAccount) string {
	if len(accs) == 0 {
		return "❌ 未找到可用下单账户"
	}
	if len(accs) == 1 {
		res := telegram.ProcessOrder(state, accs[0].ID, o.PlanCode, o.Datacenter, o.Quantity, o.Options)
		if res.Success {
			optsText := ""
			if len(o.Options) > 0 {
				optsText = fmt.Sprintf("\n⚙️ 配置: %s", strings.Join(o.Options, ", "))
			}
			return fmt.Sprintf("📥 已创建 %d/%d 个抢购任务\n\n📦 型号: %s%s\n👤 账户: %s\n\n"+
				"系统将自动重试排队到抢到为止（下单成功≠已付款）。\n查看任务 /tasks",
				res.CreatedOrders, res.TotalOrders, o.PlanCode, optsText, telegram.AccountLabel(accs[0]))
		}
		return "❌ 下单失败\n\n" + res.Message
	}

	var b strings.Builder
	okCount, total := 0, 0
	var fails []string
	for _, a := range accs {
		res := telegram.ProcessOrder(state, a.ID, o.PlanCode, o.Datacenter, o.Quantity, o.Options)
		if res.Success {
			okCount++
			total += res.CreatedOrders
			b.WriteString("  ✅ " + telegram.AccountLabel(a) +
				fmt.Sprintf(" — %d 个任务\n", res.CreatedOrders))
		} else {
			fails = append(fails, "  ❌ "+telegram.AccountLabel(a)+" — "+truncate(res.Message, 80))
		}
	}

	var head strings.Builder
	if okCount == 0 {
		head.WriteString("❌ " + strconv.Itoa(len(accs)) + " 个账户均未能下单\n\n")
	} else {
		head.WriteString(fmt.Sprintf("📥 %d/%d 个账户已开抢，共 %d 个任务\n\n", okCount, len(accs), total))
	}
	head.WriteString(b.String())
	for _, f := range fails {
		head.WriteString(f + "\n")
	}
	if okCount > 1 {
		head.WriteString("\n⚠️ 提示：多个账户正在同时抢购该型号，均抢到即为多台扣款。\n" +
			"   如需取消可在 /tasks 中一键停止。")
	}
	head.WriteString("\n查看任务列表: /tasks")
	return head.String()
}
