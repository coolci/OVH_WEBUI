package telegram

import (
	"strings"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

// kvTGActiveAccount Telegram 侧的「当前账户」。
//
// 账户只切一次，之后所有操作都跟着走。
// TG 这边原来连选都不能选 —— 文本下单和 /watch 一律落默认账户。
// 而 planCode 是分区的（EU / US / CA 三套目录基本不重合），
// 落错账户 OVH 返回的是 200 + 空数组而不是报错，表现就是"永远抢不到"，
// 用户完全看不出是账户选错了。
const kvTGActiveAccount = "telegram_active_account"

// ActiveAccount 取 Telegram 侧当前选中的账户。
//
// 返回的账户保证此刻还存在：存的 id 指向一个已删除的账户时退回默认账户 ——
// 留着一个死 id 只会让下单在几小时后才失败。
// 第二个返回值说明是不是真的按用户选择解析出来的（false = 退回了默认账户）。
func ActiveAccount(state *app.State) (types.OVHAccount, bool) {
	var id string
	if state.DB != nil {
		_, _ = state.DB.GetKV(kvTGActiveAccount, &id)
	}
	if id = strings.TrimSpace(id); id != "" {
		if acc, ok := state.FindAccount(id); ok && acc.ID == id {
			return acc, true
		}
		state.Logger.Warn("Telegram 选中的账户已不存在，退回默认账户: "+id, "telegram")
	}
	acc, ok := state.FindAccount("")
	return acc, ok && id == ""
}

// ActiveAccountID 只要 id。拿不到任何账户时返回空串，调用方按"默认账户"处理。
func ActiveAccountID(state *app.State) string {
	acc, _ := ActiveAccount(state)
	return acc.ID
}

// SetActiveAccount 切换 Telegram 侧的当前账户。
func SetActiveAccount(state *app.State, id string) error {
	if state.DB == nil {
		return nil
	}
	return state.DB.SetKV(kvTGActiveAccount, strings.TrimSpace(id))
}

// AccountLabel 给用户看的账户描述："名字（子公司 / 大区）"。
//
// 大区必须写出来：三个站点的机型目录互不相通，同一台机器在不同区是不同的 planCode。
// 只写名字的话，用户根本看不出自己选的是不是对的那一个。
func AccountLabel(acc types.OVHAccount) string {
	zone := strings.ToUpper(strings.TrimSpace(acc.Zone))
	if zone == "" {
		return acc.Name
	}
	return acc.Name + "（" + zone + " / " + ovh.SubsidiaryRegion(zone) + " 区）"
}
