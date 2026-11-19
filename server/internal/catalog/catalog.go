package catalog

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

// 多账户:用 accountID 对应账户的 zone 作 ovhSubsidiary(空 = 默认账户),不读全局 state.Config
// (新建账户不会写 kv['config'])。endpoint 与 ovhSubsidiary 一起决定目录范围,
// EU 与 US 的 nichandle.OvhSubsidiaryEnum 取值集合本身就不同,必须跟着账户走。
//
// 第二个返回值是"可用性预拉失败的 plan 数":这些 plan 的机房列会是空的,
// 调用方据此决定要不要用这份结果覆盖缓存。
func LoadServerList(state *app.State, accountID string) ([]types.ServerPlan, int) {
	client, err := state.OVH.IsolatedClientFor(accountID)
	if err != nil {
		state.Logger.Error("Failed to load server list: "+err.Error(), "")
		return nil, 0
	}
	acc, _ := state.FindAccount(accountID)
	// 子公司必须大写:三个站点都用 nichandle.OvhSubsidiaryEnum 校验,
	// 传小写 "ie" 直接 400 invalid ovhSubsidiary(EU/US/CA 实测一致),整份机型列表就空了。
	subsidiary := SubsidiaryOfAccount(acc)
	subRegion := ovh.SubsidiaryRegion(subsidiary)
	epRegion := ovh.EndpointRegion(acc.Endpoint)
	if ovh.KnownSubsidiary(subsidiary) && subRegion != epRegion {
		// 子公司与 endpoint 不在同一个站点:目录来自 A 站点、库存来自 B 站点,
		// 拼出来的机型列表每台都会是"零机房"。这种账户配置本身就是错的,直接说清楚,
		// 别让用户对着一份看似正常、实则全空的列表猜。
		state.Logger.Error(fmt.Sprintf("账户配置有误:子公司 %s 属于 %s 站点，而 endpoint %s 属于 %s 站点，"+
			"两者必须一致(EU/US/CA 是三套互不相通的系统)", subsidiary, subRegion, acc.Endpoint, epRegion), "")
		return nil, 0
	}

	// 目录站点由子公司决定(ovh.CatalogBaseURLForSubsidiary),不是由账户 endpoint 决定 ——
	// 这是同一件事的两种说法,但只有前者在账户 zone 为空、按 endpoint 兜底时也成立。
	// 走公开 URL 而不是账户凭据:这是一份 ~10MB 的公开数据,没必要占账户的 API 配额/限流额度。
	catalogResp, err := fetchEcoCatalogRaw(subsidiary)
	if err != nil {
		// 公开站点拉不到(本机出网被限、OVH 瞬断)时,退回账户凭据再拉一次,行为与老版本一致。
		state.Logger.Warn(fmt.Sprintf("公开目录拉取失败(%s/%s): %s，改用账户凭据重试", subRegion, subsidiary, err.Error()), "")
		var fallback map[string]interface{}
		cq := url.Values{}
		cq.Set("ovhSubsidiary", subsidiary)
		if err2 := client.Get("/order/catalog/public/eco?"+cq.Encode(), &fallback); err2 != nil {
			state.Logger.Error("Failed to load server list: "+err2.Error(), "")
			return nil, 0
		}
		catalogResp = fallback
	}

	plans, _ := catalogResp["plans"].([]interface{})
	result := []types.ServerPlan{}

	// 并发预拉所有 plan 的 availabilities（这是循环里唯一的网络 IO）
	// 96 个 plan × 200ms 串行 = 20 秒；改 15 并发 ≈ 1.5 秒
	type availEntry struct {
		availabilities []map[string]interface{}
		failed         bool
	}
	availByPlan := make(map[string]*availEntry, len(plans))
	var availMu sync.Mutex
	planCodes := make([]string, 0, len(plans))
	for _, planRaw := range plans {
		if p, ok := planRaw.(map[string]interface{}); ok {
			if pc := getString(p, "planCode", ""); pc != "" {
				planCodes = append(planCodes, pc)
			}
		}
	}
	// 并发从 15 降到 8:96 个 plan 同时打同一 endpoint 最容易吃 OVH 429,
	// 一旦限流就是大批 plan 同时静默变"零机房"。失败再串行重试一次兜底。
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	availFailed := 0
	var rateLimited sync.Once
	isRateLimited := false
	for _, pc := range planCodes {
		wg.Add(1)
		sem <- struct{}{}
		go func(planCode string) {
			defer wg.Done()
			defer func() { <-sem }()
			availMu.Lock()
			abort := isRateLimited
			availMu.Unlock()
			if abort {
				availMu.Lock()
				availFailed++
				availByPlan[planCode] = &availEntry{failed: true}
				availMu.Unlock()
				return
			}
			isoClient, ierr := ovh.IsolateClient(client)
			if ierr != nil {
				availMu.Lock()
				availFailed++
				availByPlan[planCode] = &availEntry{failed: true}
				availMu.Unlock()
				return
			}
			var avs []map[string]interface{}
			q := url.Values{}
			q.Set("planCode", planCode)
			err := isoClient.Get("/dedicated/server/datacenter/availabilities?"+q.Encode(), &avs)
			if err != nil {
				if strings.Contains(err.Error(), "429") || strings.Contains(strings.ToLower(err.Error()), "too many requests") {
					availMu.Lock()
					isRateLimited = true
					availFailed++
					availByPlan[planCode] = &availEntry{availabilities: avs, failed: true}
					availMu.Unlock()
					rateLimited.Do(func() {
						state.Logger.Warn(fmt.Sprintf("预拉 %s 可用性触发 OVH 429 限流，提前中止本轮剩余并发预拉", planCode), "")
					})
					return
				}
				// 瞬断退避重试一次,仍失败才计入失败数
				time.Sleep(500 * time.Millisecond)
				err = isoClient.Get("/dedicated/server/datacenter/availabilities?"+q.Encode(), &avs)
			}
			availMu.Lock()
			if err != nil {
				if strings.Contains(err.Error(), "429") || strings.Contains(strings.ToLower(err.Error()), "too many requests") {
					isRateLimited = true
				}
				availFailed++
				state.Logger.Warn(fmt.Sprintf("预拉 %s 的可用性失败: %s（该机型的机房列表将为空）", planCode, err.Error()), "")
			}
			availByPlan[planCode] = &availEntry{availabilities: avs, failed: err != nil}
			availMu.Unlock()
		}(pc)
	}
	wg.Wait()
	if availFailed > 0 {
		state.Logger.Error(fmt.Sprintf("已并发预拉 %d 个 plan 的可用性，其中 %d 个失败，这份结果的机房数据不完整",
			len(planCodes), availFailed), "")
	} else {
		state.Logger.Info(fmt.Sprintf("已并发预拉 %d 个 plan 的可用性", len(planCodes)), "")
	}

	// 建立底层 products 硬件技术字典（供 plan 依据 product 外键关联读取精确硬件规格）
	productsRaw, _ := catalogResp["products"].([]interface{})
	productByName := make(map[string]map[string]interface{}, len(productsRaw))
	for _, pr := range productsRaw {
		if pm, ok := pr.(map[string]interface{}); ok {
			if name := getString(pm, "name", ""); name != "" {
				productByName[name] = pm
			}
		}
	}

	for _, planRaw := range plans {
		plan, ok := planRaw.(map[string]interface{})
		if !ok {
			continue
		}
		planCode := getString(plan, "planCode", "")
		if planCode == "" {
			continue
		}

		// plan.product 通常是指向顶层 products 的外键(字符串),个别目录直接内联对象
		var linkedProd map[string]interface{}
		switch ref := plan["product"].(type) {
		case string:
			linkedProd = productByName[ref]
		case map[string]interface{}:
			linkedProd = ref
		}

		// 从预拉结果取并按机房码去重合并最优可用状态（同一 planCode 下常有多条 FQN）
		datacenters := []types.Datacenter{}
		dcIdxByCode := map[string]int{}
		var availabilities []map[string]interface{}
		stockStale := false
		if entry, ok := availByPlan[planCode]; ok {
			availabilities = entry.availabilities
			stockStale = entry.failed
		}
		for _, item := range availabilities {
			if dcsRaw, ok := item["datacenters"].([]interface{}); ok {
				for _, dcRaw := range dcsRaw {
					dc, ok := dcRaw.(map[string]interface{})
					if !ok {
						continue
					}
					dcCode := getString(dc, "datacenter", "")
					avail := getString(dc, "availability", "unknown")
					key := strings.ToLower(strings.TrimSpace(dcCode))
					if idx, exists := dcIdxByCode[key]; exists {
						datacenters[idx].Availability = pickBetterAvailability(datacenters[idx].Availability, avail)
						continue
					}
					dcIdxByCode[key] = len(datacenters)
					datacenters = append(datacenters, types.Datacenter{
						Datacenter:   dcCode,
						Availability: avail,
					})
				}
			}
		}
		// 填充中文名/region
		for i := range datacenters {
			if name, area, ok := lookupDCName(datacenters[i].Datacenter); ok {
				datacenters[i].DCName = name
				datacenters[i].Region = area
			} else {
				datacenters[i].DCName = datacenters[i].Datacenter
				if datacenters[i].DCName == "" {
					datacenters[i].DCName = "未知"
				}
				datacenters[i].Region = "未知"
			}
		}

		serverInfo := types.ServerPlan{
			PlanCode:         planCode,
			Name:             getString(plan, "invoiceName", ""),
			Description:      getString(plan, "description", ""),
			CPU:              "N/A",
			Memory:           "N/A",
			Storage:          "N/A",
			Bandwidth:        "N/A",
			VrackBandwidth:   "N/A",
			Datacenters:      datacenters,
			DefaultOptions:   []types.ServerOption{},
			AvailableOptions: []types.ServerOption{},
			StockStale:       stockStale,
		}

		// 优先从关联 product 的硬件表读 CPU(品牌 + 型号 + 物理处理器数量,移植上游 v0.4.17)。
		// 读不到就留 N/A,最后再从名称里找;不再编造"SK系列专用CPU"一类的系列标签 ——
		// 那会盖住后面从 details / configurations 读到的真实型号。
		serverInfo.CPU = productCPU(linkedProd)

		if serverInfo.Name == "" {
			serverInfo.Name = getString(plan, "displayName", "")
		}
		if serverInfo.Description == "" {
			serverInfo.Description = getString(plan, "displayName", "")
		}

		// 从 addonFamilies 提取硬件 + 选项
		if afRaw, ok := plan["addonFamilies"].([]interface{}); ok {
			tempAvailable := []types.ServerOption{}
			for _, familyRaw := range afRaw {
				family, ok := familyRaw.(map[string]interface{})
				if !ok {
					continue
				}
				familyName := strings.ToLower(getString(family, "name", ""))
				defaultAddon := getString(family, "default", "")
				addons, _ := family["addons"].([]interface{})

				for _, addonRaw := range addons {
					addonCode, ok := addonRaw.(string)
					if !ok {
						continue
					}
					isDefault := addonCode == defaultAddon
					lcAddon := strings.ToLower(addonCode)
					// 过滤许可证
					if strings.Contains(lcAddon, "windows-server") ||
						strings.Contains(lcAddon, "sql-server") ||
						strings.Contains(lcAddon, "cpanel-license") ||
						strings.Contains(lcAddon, "plesk-") ||
						strings.Contains(lcAddon, "-license-") ||
						strings.HasPrefix(lcAddon, "os-") ||
						strings.Contains(lcAddon, "control-panel") ||
						strings.Contains(lcAddon, "panel") {
						continue
					}
					tempAvailable = append(tempAvailable, types.ServerOption{
						Label:     addonCode,
						Value:     addonCode,
						Family:    familyName,
						IsDefault: isDefault,
					})
					if isDefault {
						serverInfo.DefaultOptions = append(serverInfo.DefaultOptions, types.ServerOption{
							Label: addonCode, Value: addonCode,
						})
					}
				}

				// 硬件信息抽取
				if defaultAddon != "" {
					switch {
					case (strings.Contains(familyName, "cpu") || strings.Contains(familyName, "processor")) && serverInfo.CPU == "N/A":
						serverInfo.CPU = defaultAddon
					case (strings.Contains(familyName, "memory") || strings.Contains(familyName, "ram")) && serverInfo.Memory == "N/A":
						if m := regexp.MustCompile(`(?i)ram-(\d+)g`).FindStringSubmatch(defaultAddon); m != nil {
							serverInfo.Memory = m[1] + " GB"
						} else {
							serverInfo.Memory = defaultAddon
						}
					case (strings.Contains(familyName, "storage") || strings.Contains(familyName, "disk") || strings.Contains(familyName, "drive") ||
						strings.Contains(familyName, "ssd") || strings.Contains(familyName, "hdd")) && serverInfo.Storage == "N/A":
						hybridRe := regexp.MustCompile(`(?i)hybridsoftraid-(\d+)x(\d+)(sa|ssd|hdd)-(\d+)x(\d+)(nvme|ssd|hdd)`)
						if m := hybridRe.FindStringSubmatch(defaultAddon); m != nil {
							serverInfo.Storage = fmt.Sprintf("混合RAID %sx %sGB %s + %sx %sGB %s",
								m[1], m[2], strings.ToUpper(m[3]), m[4], m[5], strings.ToUpper(m[6]))
						} else {
							storRe := regexp.MustCompile(`(?i)(raid|softraid)-(\d+)x(\d+)(ssd|hdd|nvme|sa)`)
							if m := storRe.FindStringSubmatch(defaultAddon); m != nil {
								serverInfo.Storage = fmt.Sprintf("%s %sx %sGB %s",
									strings.ToUpper(m[1]), m[2], m[3], strings.ToUpper(m[4]))
							} else {
								serverInfo.Storage = defaultAddon
							}
						}
					case (strings.Contains(familyName, "bandwidth") || strings.Contains(familyName, "traffic") || strings.Contains(familyName, "network")) && serverInfo.Bandwidth == "N/A":
						serverInfo.Bandwidth = parseBandwidthValue(defaultAddon, &serverInfo)
					}
				}
			}
			if len(tempAvailable) > 0 {
				serverInfo.AvailableOptions = tempAvailable
			}
		}

		// 解析方法 2: 从 plan.details.properties 提取
		if details, ok := plan["details"].(map[string]interface{}); ok {
			if propsRaw, ok := details["properties"].([]interface{}); ok {
				for _, pRaw := range propsRaw {
					prop, ok := pRaw.(map[string]interface{})
					if !ok {
						continue
					}
					propName := strings.ToLower(getString(prop, "name", ""))
					value := getString(prop, "value", "N/A")
					if value == "" || value == "N/A" {
						continue
					}
					switch {
					case (strings.Contains(propName, "cpu") || strings.Contains(propName, "processor")) && serverInfo.CPU == "N/A":
						serverInfo.CPU = value
					case (strings.Contains(propName, "memory") || strings.Contains(propName, "ram")) && serverInfo.Memory == "N/A":
						serverInfo.Memory = value
					case (strings.Contains(propName, "storage") || strings.Contains(propName, "disk") || strings.Contains(propName, "hdd") || strings.Contains(propName, "ssd")) && serverInfo.Storage == "N/A":
						serverInfo.Storage = value
					case strings.Contains(propName, "bandwidth"):
						if strings.Contains(propName, "vrack") || strings.Contains(propName, "private") || strings.Contains(propName, "internal") {
							if serverInfo.VrackBandwidth == "N/A" {
								serverInfo.VrackBandwidth = value
							}
						} else if serverInfo.Bandwidth == "N/A" {
							serverInfo.Bandwidth = value
						}
					}
				}
			}
		}

		// 解析方法 3: 从 linkedProd.configurations 提取
		if linkedProd != nil {
			if cfgs, ok := linkedProd["configurations"].([]interface{}); ok {
				for _, cRaw := range cfgs {
					cfg, ok := cRaw.(map[string]interface{})
					if !ok {
						continue
					}
					cfgName := strings.ToLower(getString(cfg, "name", ""))
					value := getString(cfg, "value", "")
					if value == "" {
						continue
					}
					switch {
					case (strings.Contains(cfgName, "cpu") || strings.Contains(cfgName, "processor")) && serverInfo.CPU == "N/A":
						serverInfo.CPU = value
					case (strings.Contains(cfgName, "memory") || strings.Contains(cfgName, "ram")) && serverInfo.Memory == "N/A":
						serverInfo.Memory = value
					case (strings.Contains(cfgName, "storage") || strings.Contains(cfgName, "disk") || strings.Contains(cfgName, "hdd") || strings.Contains(cfgName, "ssd")) && serverInfo.Storage == "N/A":
						serverInfo.Storage = value
					case strings.Contains(cfgName, "bandwidth") && serverInfo.Bandwidth == "N/A":
						serverInfo.Bandwidth = value
					}
				}
			}
		}

		// 解析方法 4: 从 plan.description 逗号分割解析
		if desc := getString(plan, "description", ""); desc != "" {
			for _, part := range strings.Split(desc, ",") {
				part = strings.ToLower(strings.TrimSpace(part))
				if part == "" {
					continue
				}
				if serverInfo.Memory == "N/A" && (strings.Contains(part, "ram") || strings.Contains(part, "gb") || strings.Contains(part, "memory")) {
					serverInfo.Memory = part
				}
				if serverInfo.Storage == "N/A" && (strings.Contains(part, "hdd") || strings.Contains(part, "ssd") || strings.Contains(part, "nvme") || strings.Contains(part, "storage") || strings.Contains(part, "disk")) {
					serverInfo.Storage = part
				}
				if serverInfo.Bandwidth == "N/A" && strings.Contains(part, "bandwidth") {
					serverInfo.Bandwidth = part
				}
			}
		}

		// 解析方法 5: 从 plan.pricing.configurations 提取
		if pricing, ok := plan["pricing"].(map[string]interface{}); ok {
			if cfgs, ok := pricing["configurations"].([]interface{}); ok {
				for _, cRaw := range cfgs {
					cfg, ok := cRaw.(map[string]interface{})
					if !ok {
						continue
					}
					cfgName := strings.ToLower(getString(cfg, "name", ""))
					value := getString(cfg, "value", "")
					if value == "" {
						continue
					}
					switch {
					case strings.Contains(cfgName, "processor") && serverInfo.CPU == "N/A":
						serverInfo.CPU = value
					case strings.Contains(cfgName, "memory") && serverInfo.Memory == "N/A":
						serverInfo.Memory = value
					case strings.Contains(cfgName, "storage") && serverInfo.Storage == "N/A":
						serverInfo.Storage = value
					}
				}
			}
		}

		// 从名称提取内存/存储（次要）
		if serverInfo.Memory == "N/A" {
			fullText := serverInfo.Name + " " + serverInfo.Description
			patterns := []string{
				`(\d+)\s*GB\s*RAM`,
				`RAM\s*(\d+)\s*GB`,
				`(\d+)\s*G\s*RAM`,
				`RAM\s*(\d+)\s*G`,
				`(\d+)\s*GB`,
			}
			for _, p := range patterns {
				if m := regexp.MustCompile(`(?i)` + p).FindStringSubmatch(fullText); m != nil {
					serverInfo.Memory = m[1] + " GB"
					break
				}
			}
		}
		if serverInfo.Storage == "N/A" {
			fullText := serverInfo.Name + " " + serverInfo.Description
			patterns := []string{
				`(\d+)\s*[xX]\s*(\d+)\s*GB\s*(SSD|HDD|NVMe)`,
				`(\d+)\s*TB\s*(SSD|HDD|NVMe)`,
				`(\d+)\s*(SSD|HDD|NVMe)`,
			}
			for _, p := range patterns {
				if m := regexp.MustCompile(`(?i)` + p).FindStringSubmatch(fullText); m != nil {
					if len(m) >= 4 && m[3] != "" {
						serverInfo.Storage = fmt.Sprintf("%sx %sGB %s", m[1], m[2], strings.ToUpper(m[3]))
					} else if len(m) >= 3 {
						serverInfo.Storage = fmt.Sprintf("%s %s", m[1], strings.ToUpper(m[2]))
					}
					break
				}
			}
		}

		if serverInfo.CPU == "N/A" {
			extractCPUFromNames(plan, &serverInfo, state)
		}
		result = append(result, serverInfo)
	}

	return result, availFailed
}

// parseBandwidthValue 解析 OVH 目录里的带宽字段
// 支持 6 种格式：traffic-Xtb-Y / traffic-Xtb / bandwidth-N（含 Gbps 转换）/ unlimited / guarantee / vrack
func parseBandwidthValue(defaultValue string, sv *types.ServerPlan) string {
	lc := strings.ToLower(defaultValue)

	// 1) traffic-Xtb-Y: X 流量 + Y Mbps
	if m := regexp.MustCompile(`(?i)traffic-(\d+)(tb|gb|mb)-(\d+)`).FindStringSubmatch(defaultValue); m != nil {
		return fmt.Sprintf("%s Mbps / %s %s流量", m[3], m[1], strings.ToUpper(m[2]))
	}
	// 2) traffic-X(tb|gb|mb)$: 仅流量限制
	if m := regexp.MustCompile(`(?i)traffic-(\d+)(tb|gb|mb)$`).FindStringSubmatch(defaultValue); m != nil {
		return fmt.Sprintf("%s %s流量", m[1], strings.ToUpper(m[2]))
	}
	// 3) bandwidth-N: 仅带宽 N Mbps，≥1000 自动转 Gbps
	if m := regexp.MustCompile(`(?i)bandwidth-(\d+)`).FindStringSubmatch(defaultValue); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			if n >= 1000 {
				gbps := float64(n) / 1000.0
				s := strconv.FormatFloat(gbps, 'f', 1, 64)
				s = strings.TrimSuffix(s, ".0")
				return s + " Gbps"
			}
			return strconv.Itoa(n) + " Mbps"
		}
		return m[1] + " Mbps"
	}
	// 4) unlimited 流量（含数字时 → "N Mbps / 无限流量"）
	if strings.Contains(lc, "traffic-unlimited") || strings.Contains(lc, "unlimited") {
		if m := regexp.MustCompile(`(\d+)`).FindStringSubmatch(defaultValue); m != nil {
			return m[1] + " Mbps / 无限流量"
		}
		return "无限流量"
	}
	// 5) guarantee / guaranteed: 保证带宽
	if strings.Contains(lc, "guarantee") || strings.Contains(lc, "guaranteed") {
		if m := regexp.MustCompile(`(\d+)`).FindStringSubmatch(defaultValue); m != nil {
			return m[1] + " Mbps (保证带宽)"
		}
		return "保证带宽"
	}
	// 6) vrack-bandwidth-X: 内部网络带宽（写到 VrackBandwidth 字段）
	if strings.Contains(lc, "vrack") {
		if m := regexp.MustCompile(`(?i)vrack-bandwidth-(\d+)`).FindStringSubmatch(defaultValue); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n >= 1000 {
				gbps := float64(n) / 1000.0
				s := strconv.FormatFloat(gbps, 'f', 1, 64)
				s = strings.TrimSuffix(s, ".0")
				sv.VrackBandwidth = s + " Gbps"
			} else {
				sv.VrackBandwidth = m[1] + " Mbps"
			}
		}
		return defaultValue
	}
	return defaultValue
}

func extractCPUFromNames(plan map[string]interface{}, info *types.ServerPlan, state *app.State) {
	displayName := getString(plan, "displayName", "")
	invoiceName := getString(plan, "invoiceName", "")
	description := getString(plan, "description", "")
	for _, name := range []string{displayName, invoiceName, description} {
		// 从品牌/系列关键词起取到分隔符为止,保留完整型号与品牌,不截断
		if cpu := cpuFromText(name); cpu != "" {
			info.CPU = cpu
			state.Logger.Info(fmt.Sprintf("从名称中提取CPU型号: %s 给 %s", cpu, info.PlanCode), "")
			return
		}
	}
}

func getString(m map[string]interface{}, key, fallback string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return fallback
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// pickBetterAvailability 合并同一机房跨多条 FQN 的可用状态：可下单状态优先于不可下单状态。
func pickBetterAvailability(cur, candidate string) string {
	curOrd := IsAvailableForOrder(cur)
	candOrd := IsAvailableForOrder(candidate)
	if curOrd && !candOrd {
		return cur
	}
	if !curOrd && candOrd {
		return candidate
	}
	if !curOrd && !candOrd {
		rank := func(s string) int {
			switch s {
			case "comingSoon":
				return 3
			case "unavailable":
				return 2
			case "unknown", "":
				return 1
			default:
				return 1
			}
		}
		if rank(candidate) > rank(cur) {
			return candidate
		}
		return cur
	}
	return cur
}
