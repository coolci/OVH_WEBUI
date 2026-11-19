package monitor

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/catalog"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

// —— 可用性查询账户的区域选取 ——
//
// 为什么需要选账户:OVH 的 EU / US / CA 是三套彼此独立的系统,
// /dedicated/server/datacenter/availabilities 在三个站点各维护一份库存视图。
// 拿不属于本站点的 planCode 去查,OVH 返回的是 **HTTP 200 + 空数组**,不是错误。
// 实测(未鉴权公开调用,三站点同一条 query):
//
//	24sk102-us  : EU 200 []      US 200 2条    CA 200 []
//	24adv01-v3  : EU 200 32条    US 200 []     CA 200 32条
//	24sk202-sgp : EU 200 4条     US 200 4条    CA 200 4条
//
// 也就是说:美区目录的 -us/-eu/-ca 后缀机型只有 US 站点查得到,
// 欧区目录的机型只有 EU/CA 站点查得到(EU 与 CA 的库存视图实测逐条一致),
// 少数亚太机型三边都有。
//
// 订阅没设 AutoOrderAccountID(只通知不下单)时,以前一律落默认账户 ——
// 默认账户是欧区、订阅的是美区 planCode,就永远返回空数组、永远不告警、也不报错。
// 现在改成:按 planCode 在各账户子公司公开目录里的归属自动挑一个区域正确的账户。

// checkAccountTTL 查询账户选取结果的缓存时长。
// 目录本身已有 120 秒缓存,这里再缓一层是为了不让「每订阅 × 每 5 秒」的检查
// 一轮轮地把所有账户遍历一遍、并反复刷同样的日志。
const checkAccountTTL = 10 * time.Minute

// transientChoiceTTL 「这轮没判出来」这类结论的缓存时长。
// 短是因为它是失败态:OVH 抖一下就该尽快重试,但也不能每 5 秒重试一次把目录接口打爆。
const transientChoiceTTL = 60 * time.Second

// queryAccountChoice 一次选取的结果
type queryAccountChoice struct {
	accountID  string // 用来查可用性(以及验价)的账户;"" = 默认账户
	region     string // EU / US / CA
	subsidiary string // ovhSubsidiary
	// degradeReason 非空 = 没能选到区域正确的账户,给用户看的中文说明。
	degradeReason string
	// definitive 为 true 表示「目录确实拉到了,而且里面没有这个 planCode」——
	// 这时再去打 OVH 只会白拿一个空数组,直接跳过。
	// 目录拉取失败时是 false:不能凭一次网络抖动就断言机型不存在。
	definitive bool
	at         time.Time
	// ttl 这条结论的缓存时长;0 = 用 checkAccountTTL。
	// "暂时判不出"的结论只缓很短一会儿,不能和确定性结论同寿命。
	ttl time.Duration
}

// effectiveTTL 这条结论能被复用多久。
func (c queryAccountChoice) effectiveTTL() time.Duration {
	if c.ttl > 0 {
		return c.ttl
	}
	return checkAccountTTL
}

var (
	queryAccountMu    sync.Mutex
	queryAccountCache = map[string]queryAccountChoice{} // planCode + 账户指纹 → 选取结果
)

// accountsFingerprint 当前账户集合的指纹(id/endpoint/zone)。
// 拼进缓存 key,这样新增/删除/改区一个账户后旧结果自动失效,
// 不需要账户 CRUD 那边记得来调一个 Invalidate —— 那种跨包的"记得调"迟早会漏。
func (m *Monitor) accountsFingerprint() string {
	m.state.AccountsMu.RLock()
	parts := make([]string, 0, len(m.state.Accounts))
	for _, a := range m.state.Accounts {
		parts = append(parts, a.ID+":"+strings.ToLower(a.Endpoint)+":"+strings.ToUpper(a.Zone))
	}
	m.state.AccountsMu.RUnlock()
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

// planInAccountCatalog 判断 planCode 是否出现在该账户子公司的公开目录里。
// 返回 (在目录里, 结论是否确定)。走 catalog 的 120 秒内存缓存,
// 不消耗账户 API 配额,也不会在热路径上现拉整份 10MB 目录。
//
// 必须把 catalog.ErrPlanNotInCatalog 和其它错误分开:前者是确定性结论
// (机型不属于这个大区,重试一万次也一样),后者只是目录暂时拉不到 ——
// 凭一次网络抖动就断言"机型不存在"会把正常订阅误停掉。
func (m *Monitor) planInAccountCatalog(accountID, planCode string) (inCatalog bool, definitive bool) {
	_, err := catalog.AddonFamiliesForPlan(m.state, accountID, planCode)
	switch {
	case err == nil:
		return true, true
	case errors.Is(err, catalog.ErrPlanNotInCatalog):
		// 注意:"不在 eco 目录"**不等于**"不属于这个大区"。
		// 实测 EU 站点 availabilities 有 244 个 planCode,eco 目录只有 99 个 ——
		// Scale / HCI / SDS / High-Grade 整条产品线都不在目录里却有真实库存。
		// 所以这里只当作"这个大区大概率不是它的归属",绝不作为确定性结论,
		// 真正的归属判定交给 catalog.RegionOfPlan(打可用性接口)。
		return false, false
	default:
		m.state.Logger.Debug(fmt.Sprintf("[monitor/region] 探测 %s 是否属于账户 %s 的目录失败(不据此判定): %s",
			planCode, accountID, err.Error()), "monitor")
		return false, false
	}
}

// probePlanRegionAccount 目录判不出归属时,用公开的可用性接口逐大区探一次,
// 命中的大区里挑一个账户来查。这条路径能覆盖 eco 目录里没有的机型(Scale/HCI 等)。
// 返回 (选中的账户, 是否命中, 三区都确认没有记录)。
// 第三个返回值才是"确定性结论"的唯一来源:只有所有候选大区都探测成功、且都没有记录,
// 才敢说这个 planCode 在已配置账户覆盖范围内不存在。探测本身失败一律不下结论。
func (m *Monitor) probePlanRegionAccount(planCode string, candidates []types.OVHAccount) (queryAccountChoice, bool, bool) {
	regions := make([]string, 0, 3)
	for _, acc := range candidates {
		r, _ := accountRegionInfo(acc)
		regions = append(regions, r)
	}
	region, err := catalog.RegionOfPlan(m.state, planCode, regions)
	if err != nil {
		return queryAccountChoice{}, false, false
	}
	if region == "" {
		return queryAccountChoice{}, false, true // 探测都成功了,确实哪个区都没有
	}
	for _, acc := range candidates {
		r, subsidiary := accountRegionInfo(acc)
		if r != region {
			continue
		}
		m.state.Logger.Info(fmt.Sprintf("[monitor/region] %s 未出现在 eco 目录,但 %s 站点的可用性接口有记录,用账户「%s」查询",
			planCode, region, acc.Name), "monitor")
		return queryAccountChoice{accountID: acc.ID, region: region, subsidiary: subsidiary, at: time.Now()}, true, false
	}
	return queryAccountChoice{}, false, false
}

// explainEmptyAvailability 查询账户明明选出来了、可用性接口却返回空时,给一个说得清的原因。
//
// 为什么要多探这一下:OVH 三个站点互不相通,拿别站点的 planCode 去查是 HTTP 200 + 空数组
// (实测 24sk102-us 在 EU/CA 都是 0 条、US 是 2 条),和"这台机器真的下架了"长得一模一样。
// 只有把"别的大区能查到记录"这件事查出来,用户才知道该去改订阅的账户而不是干等补货。
//
// 探测走 catalog.RegionOfPlan(公开可用性接口 + 10 分钟缓存,不消耗账户配额),
// 候选大区只取已配置账户覆盖到的那些 —— 本项目不另外维护大区枚举,
// 想覆盖"你压根没有该大区账户"的情况需要 ovh 包提供全量大区列表(见 needsOther)。
func (m *Monitor) explainEmptyAvailability(planCode string, choice queryAccountChoice) string {
	generic := fmt.Sprintf("已用 %s 站点(子公司 %s)查询,OVH 未返回 %s 的任何配置组合(机型可能已下架,或 OVH 接口暂时异常)",
		choice.region, choice.subsidiary, planCode)

	// 本轮用的大区排第一:它刚刚已经被证实是空的,让 RegionOfPlan 先确认这一点,
	// 命中别的大区才说明是区域错配。
	regions := []string{choice.region}
	m.state.AccountsMu.RLock()
	for _, a := range m.state.Accounts {
		r, _ := accountRegionInfo(a)
		regions = append(regions, r)
	}
	m.state.AccountsMu.RUnlock()

	region, err := catalog.RegionOfPlan(m.state, planCode, regions)
	if err != nil || region == "" || region == choice.region {
		// 探测失败 / 哪个区都没有 / 就在本区(那就是真的没数据)—— 都不改口径,给通用说明
		return generic
	}
	return fmt.Sprintf(
		"机型 %s 的库存记录只在 %s 站点查得到,而本轮用的是 %s 站点(子公司 %s)的账户。"+
			"OVH 的 EU/US/CA 三套系统互不相通,这条订阅在当前账户下既查不到库存也下不了单,"+
			"请把订阅的下单账户改成 %s 大区的账户",
		planCode, region, choice.region, choice.subsidiary, region)
}

// accountRegionInfo 取账户的大区与子公司。子公司口径统一走 catalog.SubsidiaryOfAccount,
// 大区归属统一走 ovh.SubsidiaryRegion —— 全项目就这一份权威表,这里不另立映射。
func accountRegionInfo(acc types.OVHAccount) (region, subsidiary string) {
	subsidiary = catalog.SubsidiaryOfAccount(acc)
	return ovh.SubsidiaryRegion(subsidiary), subsidiary
}

// resolveQueryAccount 决定这条订阅该用哪个账户去查可用性 / 验价。
//
// 优先级:
//  1. 订阅显式指定了 AutoOrderAccountID → 用它。验价账户和下单账户必须是同一个,
//     否则「A 账户验得出价、B 账户下不了单」。账户不存在时只能报错,不能悄悄改用默认账户。
//  2. 否则按 planCode 在各账户子公司目录里的归属挑:默认账户优先(保持既有行为),
//     其次按账户顺序找第一个目录里有这个 plan 的。
//  3. 一个都没有 → 用可用性接口逐大区探,再不行落默认账户并给出明确的降级说明。
//
// 参数是 (planCode, autoOrderAccountID) 而不是 *Subscription:这两个值必须由调用方
// 在持订阅锁时取好(见 Subscription 的并发约定),而且 PreflightRegion 手里根本没有订阅对象。
func (m *Monitor) resolveQueryAccount(planCode, autoOrderAccountID string) queryAccountChoice {
	// 缓存 key 里带上 autoOrderAccountID 和账户指纹:
	// 订阅改了下单账户 → key 变 → 立刻生效;账户增删改区 → 指纹变 → 立刻失效。
	// 所以显式账户这条路径同样可以吃缓存,不必像以前那样每订阅每 5 秒重算一遍
	// (那条路径里的 planInAccountCatalog 在目录缓存过期时会现拉一份整目录)。
	cacheKey := planCode + "@" + autoOrderAccountID + "@" + m.accountsFingerprint()
	queryAccountMu.Lock()
	if c, ok := queryAccountCache[cacheKey]; ok && time.Since(c.at) < c.effectiveTTL() {
		queryAccountMu.Unlock()
		return c
	}
	queryAccountMu.Unlock()

	cache := func(c queryAccountChoice) queryAccountChoice {
		c.at = time.Now()
		queryAccountMu.Lock()
		queryAccountCache[cacheKey] = c
		queryAccountMu.Unlock()
		return c
	}

	if autoOrderAccountID != "" {
		acc, ok := m.state.FindAccount(autoOrderAccountID)
		if !ok {
			return cache(queryAccountChoice{
				degradeReason: fmt.Sprintf("订阅指定的下单账户 %s 已不存在,无法确定该用哪个大区查询库存,请重新选择账户",
					autoOrderAccountID),
			})
		}
		region, subsidiary := accountRegionInfo(acc)
		// 这里**不再**用 eco 目录判"机型是不是属于这个账户的大区":
		// 实测 EU 站点 availabilities 有 244 个 planCode、eco 目录只有 99 个,
		// Scale/HCI/SDS/High-Grade 整条线不在目录里却有真实库存 ——
		// 按目录判会把这些订阅打上"区域错配"的假警报。
		// 真正的错配留到查出空结果时由 explainEmptyAvailability 用可用性接口坐实。
		return cache(queryAccountChoice{accountID: acc.ID, region: region, subsidiary: subsidiary})
	}

	m.state.AccountsMu.RLock()
	accounts := make([]types.OVHAccount, len(m.state.Accounts))
	copy(accounts, m.state.Accounts)
	m.state.AccountsMu.RUnlock()

	if len(accounts) == 0 {
		return cache(queryAccountChoice{degradeReason: "未配置任何 OVH 账户,无法查询库存"})
	}

	// 候选顺序:默认账户排最前,保证「plan 在默认账户大区里」时行为与以前完全一致
	candidates := make([]types.OVHAccount, 0, len(accounts))
	for _, a := range accounts {
		if a.IsDefault {
			candidates = append(candidates, a)
		}
	}
	for _, a := range accounts {
		if !a.IsDefault {
			candidates = append(candidates, a)
		}
	}

	triedRegions := map[string]struct{}{}
	for _, acc := range candidates {
		region, subsidiary := accountRegionInfo(acc)
		triedRegions[region] = struct{}{}
		// planInAccountCatalog 只有"确实在目录里"时才返回 (true, true),
		// 其余情况(不在目录 / 拉取失败)都是不确定 —— 见该函数上的说明。
		// 所以这里只把它当作"快速命中"的捷径,判不出来一律交给下面的可用性探测。
		if in, definitive := m.planInAccountCatalog(acc.ID, planCode); !definitive || !in {
			continue
		}
		m.state.Logger.Info(fmt.Sprintf("[monitor/region] %s 归属 %s 站点(子公司 %s),用账户「%s」查询库存",
			planCode, region, subsidiary, acc.Name), "monitor")
		return cache(queryAccountChoice{accountID: acc.ID, region: region, subsidiary: subsidiary})
	}

	// 目录里找不到 —— 先用公开的可用性接口逐大区探一次再下结论。
	// eco 目录只覆盖能用 /order/cart/eco 下单的机型,而监控要覆盖的机型远不止这些。
	c, hit, probedAbsent := m.probePlanRegionAccount(planCode, candidates)
	if hit {
		return cache(c)
	}
	// 确定性结论只认探测结果:三区都探成功且都没有记录,才允许停止每 5 秒白打 OVH
	allDefinitive := probedAbsent

	// 没有任何账户的大区目录里有这个 plan
	regions := make([]string, 0, len(triedRegions))
	for r := range triedRegions {
		regions = append(regions, r)
	}
	sort.Strings(regions)
	choice := queryAccountChoice{}
	if allDefinitive {
		choice.definitive = true
		choice.degradeReason = fmt.Sprintf(
			"机型 %s 在已配置账户覆盖的大区(%s)里既不在目录、可用性接口也查不到任何记录。"+
				"OVH 的 EU/US/CA 三个站点互不相通(带 -us/-eu/-ca 后缀的机型只在对应站点出售),"+
				"请确认 planCode 是否拼写正确,或添加对应大区的账户",
			planCode, strings.Join(regions, "/"))
	} else {
		// 目录 / 探测没跑成,不能下确定性结论:退回默认账户按老路走一次,过一小会儿再判。
		// 这里必须用短 TTL —— 以前和成功结论一样缓 10 分钟,等于让一次网络抖动
		// 把"暂时判不出"钉死 10 分钟,注释写着"下轮再判"其实下轮读的还是这条缓存。
		choice.ttl = transientChoiceTTL
		choice.degradeReason = fmt.Sprintf(
			"暂时无法确定机型 %s 属于哪个大区(公开目录/可用性探测暂时失败),本轮退回默认账户查询", planCode)
	}
	return cache(choice)
}

// PreflightRegion 添加订阅时的区域预检:告诉调用方这个 planCode 会由哪个大区的账户来查,
// 以及有没有明显的区域错配。返回的 warning 非空时只是提示,不阻止订阅 ——
// 目录有 120 秒缓存、OVH 也会临时抖动,不能凭一次探测就把用户挡在门外。
//
// autoOrderAccountID 传订阅要用的下单账户(可为空),口径与真正检查时完全一致。
func (m *Monitor) PreflightRegion(planCode, autoOrderAccountID string) (region, subsidiary, warning string) {
	c := m.resolveQueryAccount(planCode, autoOrderAccountID)
	return c.region, c.subsidiary, c.degradeReason
}

// PlanAccount 给定 planCode，解析出「哪个账户能买到它」。
//
// 为什么下单要走这个而不是让用户自己选账户：
// 用户真正在意的从来不是"用哪个账户"，而是"别落到一个买不到这台机器的账户上"。
// 而 planCode 本身就带着区域信息 —— OVH 的 EU / US / CA 是三套彼此独立的系统，
// 同一台机器在不同区是不同的型号代码（美区通常带 -us 后缀）。
// 拿欧区 planCode 打美区接口，OVH 回的是 200 + 空数组而**不是报错**，
// 表现就是"永远抢不到"，日志里也没有异常，用户完全看不出是账户选错了。
//
// 所以账户是可以算出来的，不该让人记着自己上次切到哪个区了。
//
// prefer 非空且它所在大区确实有这个 planCode 时优先用它 ——
// 这是给"同一个大区有好几个账户"准备的，那种情况算不出来该用哪个。
//
// 返回值：accountID 为空表示没找到能买的账户，reason 里是给用户看的中文说明。
func (m *Monitor) PlanAccount(planCode, prefer string) (accountID, region, subsidiary, reason string) {
	// prefer 先验一遍:它所在大区有这个 plan 才认,否则当没传 ——
	// 否则又退化成"用户选了什么就用什么",区域错配照样发生。
	if prefer != "" {
		if acc, ok := m.state.FindAccount(prefer); ok {
			if in, definitive := m.planInAccountCatalog(acc.ID, planCode); definitive && in {
				r, sub := accountRegionInfo(acc)
				return acc.ID, r, sub, ""
			}
		}
	}
	c := m.resolveQueryAccount(planCode, "")
	return c.accountID, c.region, c.subsidiary, c.degradeReason
}

// PlanAccountFast 只查已缓存的目录,不做跨区可用性探测。
//
// 给交互式路径用(Telegram 下单):探测要逐个大区打 OVH,实测能到好几秒,
// 而补货那一刻每一秒都算数。目录本身有 120 秒缓存,命中就是零延迟。
//
// 判不出来时返回空 accountID —— 调用方**不要**据此拒绝下单,
// 退回当前账户并把"没能确认"这件事说出来就行:
// 判不出来的原因往往只是目录还没热,而不是真的没有合适的账户。
func (m *Monitor) PlanAccountFast(planCode, prefer string) (accountID, region, subsidiary string) {
	try := func(acc types.OVHAccount) (string, string, string, bool) {
		if in, definitive := m.planInAccountCatalog(acc.ID, planCode); definitive && in {
			r, sub := accountRegionInfo(acc)
			return acc.ID, r, sub, true
		}
		return "", "", "", false
	}
	if prefer != "" {
		if acc, ok := m.state.FindAccount(prefer); ok {
			if id, r, sub, hit := try(acc); hit {
				return id, r, sub
			}
		}
	}
	m.state.AccountsMu.RLock()
	accounts := make([]types.OVHAccount, len(m.state.Accounts))
	copy(accounts, m.state.Accounts)
	m.state.AccountsMu.RUnlock()
	// 默认账户优先,保证单账户/同区用户的行为和以前完全一致
	for _, a := range accounts {
		if !a.IsDefault {
			continue
		}
		if id, r, sub, hit := try(a); hit {
			return id, r, sub
		}
	}
	for _, a := range accounts {
		if a.IsDefault {
			continue
		}
		if id, r, sub, hit := try(a); hit {
			return id, r, sub
		}
	}
	return "", "", ""
}

// AccountsInRegion 某个大区下的所有账户。
// 同区多账户时得让用户挑一次 —— 那种情况光看 planCode 算不出来用哪个。
func (m *Monitor) AccountsInRegion(region string) []types.OVHAccount {
	m.state.AccountsMu.RLock()
	defer m.state.AccountsMu.RUnlock()
	out := []types.OVHAccount{}
	for _, a := range m.state.Accounts {
		if r, _ := accountRegionInfo(a); r == region {
			out = append(out, a)
		}
	}
	return out
}

// AccountsForPlan 所有目录里有这个 planCode 的账户。
//
// 用来做两件事：
//   - `@all`：同区每个能买的账户各下一单。抢稀缺机器时这是多账户真正的价值 ——
//     以前不管怎么切都只能用一个。
//   - 上架通知的按钮：每个机房 × 每个能买的账户各一颗，看到补货直接选账户下单。
//
// 只认目录（权威），不解析 planCode 后缀 —— 后缀推不出账户：
// 实测美区目录里 42 个 `-eu` 后缀的机型，卖的是欧洲机房的机器，
// 但要用**美区**账户下单（它们的 region 配置恒为 united_states）。
func (m *Monitor) AccountsForPlan(planCode string) []types.OVHAccount {
	m.state.AccountsMu.RLock()
	accounts := make([]types.OVHAccount, len(m.state.Accounts))
	copy(accounts, m.state.Accounts)
	m.state.AccountsMu.RUnlock()

	out := []types.OVHAccount{}
	for _, a := range accounts {
		if in, definitive := m.planInAccountCatalog(a.ID, planCode); definitive && in {
			out = append(out, a)
		}
	}
	return out
}
