package catalog

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

// CheckServerAvailabilityWithConfigs 对应 Python: check_server_availability_with_configs
// 返回每个配置组合的可用性 + 匹配到的 API2 options
type ConfigAvailability struct {
	Memory      string            `json:"memory"`
	Storage     string            `json:"storage"`
	Datacenters map[string]string `json:"datacenters"`
	FQN         string            `json:"fqn"`
	Options     []string          `json:"options"`
}

// CheckServerAvailabilityWithConfigs 返回每个配置组合的可用性 + 匹配到的 API2 options。
//   - accountID:决定用哪个账户的 OVH client 和 zone 拉 catalog。空 = 默认账户。
//     `/dedicated/server/datacenter/availabilities` 是全局接口,client 走哪个账户无所谓;
//     但 `/order/catalog/public/eco` 必须用对应 subsidiary 拉,否则跨子公司账户的 options 匹配会失败。
//   - monitor 检查 loop 没有"当前账户"概念,直接传 "",意味着只能保证默认账户 + 同 subsidiary 账户准确;
//     quick-order / Telegram 这种已知 account_id 的调用方应该传具体 ID。
func CheckServerAvailabilityWithConfigs(state *app.State, planCode string, accountID string) map[string]*ConfigAvailability {
	client, err := state.OVH.ClientFor(accountID)
	if err != nil {
		return map[string]*ConfigAvailability{}
	}

	state.Logger.Info(fmt.Sprintf("[配置监控] 查询 %s 的所有配置组合...", planCode), "monitor")

	var availabilities []map[string]interface{}
	q := url.Values{}
	q.Set("planCode", planCode)
	if err := client.Get("/dedicated/server/datacenter/availabilities?"+q.Encode(), &availabilities); err != nil {
		state.Logger.Error(fmt.Sprintf("[配置监控] 获取配置可用性失败: %s", err.Error()), "monitor")
		return map[string]*ConfigAvailability{}
	}

	if len(availabilities) == 0 {
		state.Logger.Warn(fmt.Sprintf("[配置监控] 未获取到 %s 的可用性数据", planCode), "monitor")
		return map[string]*ConfigAvailability{}
	}

	state.Logger.Info(fmt.Sprintf("[配置监控] OVH API 返回 %d 个配置组合", len(availabilities)), "monitor")

	// 取目录用于匹配 API2 选项:用指定账户的 subsidiary
	acc, _ := state.FindAccount(accountID)
	subsidiary := acc.Zone
	if subsidiary == "" {
		subsidiary = "IE"
	}
	var catalogResp map[string]interface{}
	_ = client.Get("/order/catalog/public/eco?ovhSubsidiary="+subsidiary, &catalogResp)

	result := map[string]*ConfigAvailability{}
	for _, item := range availabilities {
		memory := getString(item, "memory", "N/A")
		storage := getString(item, "storage", "N/A")
		fqn := getString(item, "fqn", "")
		configKey := fqn

		datacenters := map[string]string{}
		if dcsRaw, ok := item["datacenters"].([]interface{}); ok {
			for _, dcRaw := range dcsRaw {
				dc, ok := dcRaw.(map[string]interface{})
				if !ok {
					continue
				}
				dcName := getString(dc, "datacenter", "")
				availability := getString(dc, "availability", "unknown")
				if dcName != "" {
					datacenters[dcName] = availability
				}
			}
		}

		// 匹配 API2 options
		api2Options := []string{}
		memoryStd := ""
		storageStd := ""
		if memory != "N/A" {
			memoryStd = StandardizeConfig(memory)
		}
		if storage != "N/A" {
			storageStd = StandardizeConfig(storage)
		}

		state.Logger.Debug(fmt.Sprintf("[配置监控] 提取选项: memory=%s (标准化: %s), storage=%s (标准化: %s)",
			memory, memoryStd, storage, storageStd), "monitor")

		if (memoryStd != "" || storageStd != "") && catalogResp != nil {
			if plans, ok := catalogResp["plans"].([]interface{}); ok {
				planFound := false
				for _, planRaw := range plans {
					plan, ok := planRaw.(map[string]interface{})
					if !ok {
						continue
					}
					if getString(plan, "planCode", "") != planCode {
						continue
					}
					planFound = true
					addonFamilies, _ := plan["addonFamilies"].([]interface{})
					for _, familyRaw := range addonFamilies {
						family, ok := familyRaw.(map[string]interface{})
						if !ok {
							continue
						}
						familyName := strings.ToLower(getString(family, "name", ""))
						addons, _ := family["addons"].([]interface{})

						if familyName == "memory" && memoryStd != "" {
							for _, addonRaw := range addons {
								addon, ok := addonRaw.(string)
								if !ok {
									continue
								}
								addonStd := StandardizeConfig(addon)
								if addonStd == memoryStd {
									if !contains(api2Options, addon) {
										api2Options = append(api2Options, addon)
									}
								} else if memoryStd != "" && strings.Contains(addonStd, memoryStd) {
									if !contains(api2Options, addon) {
										api2Options = append(api2Options, addon)
									}
								}
							}
						} else if familyName == "storage" && storageStd != "" {
							for _, addonRaw := range addons {
								addon, ok := addonRaw.(string)
								if !ok {
									continue
								}
								addonStd := StandardizeConfig(addon)
								if addonStd == storageStd {
									if !contains(api2Options, addon) {
										api2Options = append(api2Options, addon)
									}
								} else if storageStd != "" && strings.Contains(addonStd, storageStd) {
									if !contains(api2Options, addon) {
										api2Options = append(api2Options, addon)
									}
								}
							}
						}
					}
					break
				}
				if !planFound {
					state.Logger.Warn(fmt.Sprintf("[配置监控] 在 catalog 中未找到 planCode: %s", planCode), "monitor")
				}
			}
		}

		if len(api2Options) > 0 {
			state.Logger.Info(fmt.Sprintf("[配置监控] 成功提取 %d 个API2选项: %v", len(api2Options), api2Options), "monitor")
		}

		result[configKey] = &ConfigAvailability{
			Memory:      memory,
			Storage:     storage,
			Datacenters: datacenters,
			FQN:         fqn,
			Options:     api2Options,
		}
		state.Logger.Info(fmt.Sprintf("[配置监控] 配置: %s + %s, 数据中心数: %d", memory, storage, len(datacenters)), "monitor")
	}

	state.Logger.Info(fmt.Sprintf("[配置监控] 成功获取 %d 个配置组合的可用性", len(result)), "monitor")
	return result
}

// CheckServerAvailability 对应 Python: check_server_availability（带 options 精确匹配）
func CheckServerAvailability(state *app.State, planCode string, options []string) (map[string]string, error) {
	client, err := state.OVH.ClientFor("")
	if err != nil {
		return nil, err
	}

	state.Logger.Info(fmt.Sprintf("查询 %s 的可用性...", planCode), "")

	var availabilities []map[string]interface{}
	q := url.Values{}
	q.Set("planCode", planCode)
	if err := client.Get("/dedicated/server/datacenter/availabilities?"+q.Encode(), &availabilities); err != nil {
		state.Logger.Error(fmt.Sprintf("Failed to check availability for %s: %s", planCode, err.Error()), "")
		return nil, err
	}

	state.Logger.Info(fmt.Sprintf("OVH API 返回 %d 个配置组合", len(availabilities)), "")
	if len(availabilities) == 0 {
		state.Logger.Warn(fmt.Sprintf("未获取到 %s 的可用性数据", planCode), "")
		return map[string]string{}, nil
	}

	// 用户提供配置：精确匹配
	if len(options) > 0 {
		state.Logger.Info(fmt.Sprintf("查询 %s 的配置选项可用性: %v", planCode, options), "")
		var memoryOption, storageOption string
		for _, opt := range options {
			optLower := strings.ToLower(opt)
			if strings.Contains(optLower, "ram-") || strings.Contains(optLower, "memory") {
				memoryOption = opt
				state.Logger.Info(fmt.Sprintf("识别内存配置: %s", opt), "")
			} else if strings.Contains(optLower, "softraid-") || strings.Contains(optLower, "hybrid") ||
				strings.Contains(optLower, "disk") || strings.Contains(optLower, "nvme") || strings.Contains(optLower, "raid") {
				storageOption = opt
				state.Logger.Info(fmt.Sprintf("识别存储配置: %s", opt), "")
			}
		}

		state.Logger.Info(fmt.Sprintf("提取配置 - 内存: %s, 存储: %s", memoryOption, storageOption), "")

		var matchedConfig map[string]interface{}
		for _, item := range availabilities {
			itemMemory := getString(item, "memory", "")
			itemStorage := getString(item, "storage", "")
			itemFQN := getString(item, "fqn", "")

			state.Logger.Info(fmt.Sprintf("检查配置: %s", itemFQN), "")
			state.Logger.Info(fmt.Sprintf("  OVH内存: %s, OVH存储: %s", itemMemory, itemStorage), "")

			memoryMatch := true
			if memoryOption != "" {
				if itemMemory != "" {
					userParts := strings.Split(memoryOption, "-")
					if len(userParts) > 2 {
						userParts = userParts[:2]
					}
					ovhParts := strings.Split(itemMemory, "-")
					if len(ovhParts) > 2 {
						ovhParts = ovhParts[:2]
					}
					userKey := strings.Join(userParts, "-")
					ovhKey := strings.Join(ovhParts, "-")
					memoryMatch = userKey == ovhKey
					state.Logger.Info(fmt.Sprintf("  内存匹配: '%s' (%s) vs '%s' (%s) = %v",
						memoryOption, userKey, itemMemory, ovhKey, memoryMatch), "")
				} else {
					memoryMatch = false
				}
			}

			storageMatch := true
			if storageOption != "" {
				if itemStorage != "" {
					storageMatch = strings.HasPrefix(storageOption, itemStorage)
					state.Logger.Info(fmt.Sprintf("  存储匹配: '%s'.startswith('%s') = %v",
						storageOption, itemStorage, storageMatch), "")
				} else {
					storageMatch = false
				}
			}

			state.Logger.Info(fmt.Sprintf("  最终匹配结果: memory=%v, storage=%v", memoryMatch, storageMatch), "")

			if memoryMatch && storageMatch {
				matchedConfig = item
				state.Logger.Info(fmt.Sprintf("✅ 找到匹配配置: %s", itemFQN), "")
				break
			}
		}

		if matchedConfig != nil {
			result := map[string]string{}
			if dcsRaw, ok := matchedConfig["datacenters"].([]interface{}); ok {
				for _, dcRaw := range dcsRaw {
					dc, ok := dcRaw.(map[string]interface{})
					if !ok {
						continue
					}
					dcName := getString(dc, "datacenter", "")
					availability := getString(dc, "availability", "unknown")
					if dcName == "" {
						continue
					}
					if availability == "" || availability == "unknown" {
						result[dcName] = "unknown"
					} else if availability == "unavailable" {
						result[dcName] = "unavailable"
					} else {
						result[dcName] = availability
					}
				}
			}
			state.Logger.Info(fmt.Sprintf("配置 %s 的可用性: %v", getString(matchedConfig, "fqn", ""), result), "")
			return result, nil
		}

		state.Logger.Warn(fmt.Sprintf("❌ 未找到匹配的配置组合！请求: %v", options), "")
		fqns := []string{}
		for _, item := range availabilities {
			fqns = append(fqns, getString(item, "fqn", ""))
		}
		state.Logger.Info(fmt.Sprintf("可用的配置组合: %v", fqns), "")
		return map[string]string{}, nil
	}

	// 未指定 options：使用第一个
	defaultConfig := availabilities[0]
	defaultFQN := getString(defaultConfig, "fqn", "")
	state.Logger.Info(fmt.Sprintf("使用默认配置: %s", defaultFQN), "")

	result := map[string]string{}
	if dcsRaw, ok := defaultConfig["datacenters"].([]interface{}); ok {
		for _, dcRaw := range dcsRaw {
			dc, ok := dcRaw.(map[string]interface{})
			if !ok {
				continue
			}
			dcName := getString(dc, "datacenter", "")
			availability := getString(dc, "availability", "unknown")
			if dcName == "" {
				continue
			}
			if availability == "" || availability == "unknown" {
				result[dcName] = "unknown"
			} else if availability == "unavailable" {
				result[dcName] = "unavailable"
			} else {
				result[dcName] = availability
			}
		}
	}
	state.Logger.Info(fmt.Sprintf("默认配置 %s 的可用性: %v", defaultFQN, result), "")
	return result, nil
}

// 数据中心名称映射 (与 Python load_server_list 一致)
var dcNameMap = map[string][2]string{
	"gra": {"格拉夫尼茨", "法国"},
	"sbg": {"斯特拉斯堡", "法国"},
	"rbx": {"鲁贝", "法国"},
	"bhs": {"博阿尔诺", "加拿大"},
	"hil": {"俄勒冈", "美国西部"},
	"vin": {"弗吉尼亚", "美国东部"},
	"lim": {"利马索尔", "塞浦路斯"},
	"sgp": {"新加坡", "新加坡"},
	"syd": {"悉尼", "澳大利亚"},
	"waw": {"华沙", "波兰"},
	"fra": {"法兰克福", "德国"},
	"lon": {"伦敦", "英国"},
	"eri": {"厄斯沃尔", "英国"},
}

// LoadServerList 对应 Python: load_server_list。
// 多账户:用默认账户的 zone 作 ovhSubsidiary,不读全局 state.Config(新建账户不会写 kv['config'])
func LoadServerList(state *app.State) []types.ServerPlan {
	client, err := state.OVH.ClientFor("")
	if err != nil {
		state.Logger.Error("Failed to load server list: "+err.Error(), "")
		return nil
	}
	acc, _ := state.FindAccount("")
	subsidiary := acc.Zone
	if subsidiary == "" {
		subsidiary = "IE"
	}

	var catalogResp map[string]interface{}
	if err := client.Get("/order/catalog/public/eco?ovhSubsidiary="+subsidiary, &catalogResp); err != nil {
		state.Logger.Error("Failed to load server list: "+err.Error(), "")
		return nil
	}

	plans, _ := catalogResp["plans"].([]interface{})
	result := []types.ServerPlan{}

	// 并发预拉所有 plan 的 availabilities（这是循环里唯一的网络 IO）
	// 96 个 plan × 200ms 串行 = 20 秒；改 15 并发 ≈ 1.5 秒
	type availEntry struct {
		availabilities []map[string]interface{}
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
	sem := make(chan struct{}, 15)
	var wg sync.WaitGroup
	for _, pc := range planCodes {
		wg.Add(1)
		sem <- struct{}{}
		go func(planCode string) {
			defer wg.Done()
			defer func() { <-sem }()
			var avs []map[string]interface{}
			q := url.Values{}
			q.Set("planCode", planCode)
			_ = client.Get("/dedicated/server/datacenter/availabilities?"+q.Encode(), &avs)
			availMu.Lock()
			availByPlan[planCode] = &availEntry{availabilities: avs}
			availMu.Unlock()
		}(pc)
	}
	wg.Wait()
	state.Logger.Info(fmt.Sprintf("已并发预拉 %d 个 plan 的可用性", len(planCodes)), "")

	for _, planRaw := range plans {
		plan, ok := planRaw.(map[string]interface{})
		if !ok {
			continue
		}
		planCode := getString(plan, "planCode", "")
		if planCode == "" {
			continue
		}

		// 从预拉结果取（保持串行解析以确保 1:1）
		datacenters := []types.Datacenter{}
		var availabilities []map[string]interface{}
		if entry, ok := availByPlan[planCode]; ok {
			availabilities = entry.availabilities
		}
		for _, item := range availabilities {
			if dcsRaw, ok := item["datacenters"].([]interface{}); ok {
				for _, dcRaw := range dcsRaw {
					dc, ok := dcRaw.(map[string]interface{})
					if !ok {
						continue
					}
					datacenters = append(datacenters, types.Datacenter{
						Datacenter:   getString(dc, "datacenter", ""),
						Availability: getString(dc, "availability", "unknown"),
					})
				}
			}
		}
		// 填充中文名/region
		for i := range datacenters {
			code := strings.ToLower(datacenters[i].Datacenter)
			if len(code) > 3 {
				code = code[:3]
			}
			if names, ok := dcNameMap[code]; ok {
				datacenters[i].DCName = names[0]
				datacenters[i].Region = names[1]
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
		}

		// 特殊系列：SYSLE / SK
		lcPlan := strings.ToLower(planCode)
		if strings.Contains(lcPlan, "sysle") {
			state.Logger.Info(fmt.Sprintf("检测到SYSLE系列服务器: %s", planCode), "")
			switch {
			case strings.Contains(planCode, "011"):
				serverInfo.CPU = "SYSLE 011系列 (入门级服务器CPU)"
			case strings.Contains(planCode, "021"):
				serverInfo.CPU = "SYSLE 021系列 (中端服务器CPU)"
			case strings.Contains(planCode, "031"):
				serverInfo.CPU = "SYSLE 031系列 (高端服务器CPU)"
			default:
				serverInfo.CPU = "SYSLE系列CPU"
			}
			extractCPUFromNames(plan, &serverInfo, state)
		} else if strings.Contains(lcPlan, "sk") {
			state.Logger.Info(fmt.Sprintf("检测到SK系列服务器: %s", planCode), "")
			displayName := getString(plan, "displayName", "")
			invoiceName := getString(plan, "invoiceName", "")
			description := getString(plan, "description", "")
			foundCPU := false
			for _, name := range []string{displayName, invoiceName, description} {
				if name == "" {
					continue
				}
				if strings.Contains(name, "|") {
					parts := strings.SplitN(name, "|", 2)
					if len(parts) > 1 {
						cpuPart := strings.TrimSpace(parts[1])
						lc := strings.ToLower(cpuPart)
						if strings.Contains(lc, "intel") || strings.Contains(lc, "amd") || strings.Contains(lc, "xeon") || strings.Contains(lc, "i7") {
							serverInfo.CPU = cpuPart
							state.Logger.Info(fmt.Sprintf("从名称中提取CPU型号: %s 给 %s", cpuPart, planCode), "")
							foundCPU = true
						}
					}
				}
				if foundCPU {
					break
				}
			}
			if !foundCPU {
				serverInfo.CPU = "SK系列专用CPU"
			}
		}

		if serverInfo.CPU == "N/A" {
			state.Logger.Info(fmt.Sprintf("服务器 %s 无法从API提取CPU信息，尝试从名称提取", planCode), "")
			extractCPUFromNames(plan, &serverInfo, state)
			if serverInfo.CPU == "N/A" {
				switch {
				case strings.Contains(lcPlan, "sysle"):
					serverInfo.CPU = "SYSLE系列专用CPU"
				case strings.Contains(lcPlan, "rise"):
					serverInfo.CPU = "RISE系列专用CPU"
				case strings.Contains(lcPlan, "game"):
					serverInfo.CPU = "GAME系列专用CPU"
				default:
					serverInfo.CPU = "专用服务器CPU"
				}
			}
		}

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
						strings.Contains(familyName, "ssd") || strings.Contains(familyName, "hdd") || strings.Contains(familyName, "nvme") ||
						strings.Contains(familyName, "sas")) && serverInfo.Storage == "N/A":
						// 统一解析：SAS 优先于 SA；hybrid 任意介质顺序；0disk
						if disp := ParseStorageAddonDisplay(defaultAddon); disp != "" {
							serverInfo.Storage = disp
						} else {
							serverInfo.Storage = defaultAddon
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

		// 解析方法 2: 从 plan.details.properties 提取（1:1 对应 app.py:2010-2040）
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

		// 解析方法 3: 从 plan.product.configurations 提取（1:1 对应 app.py:2154-2184）
		if product, ok := plan["product"].(map[string]interface{}); ok {
			if cfgs, ok := product["configurations"].([]interface{}); ok {
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

		// 解析方法 4: 从 plan.description 逗号分割解析（1:1 对应 app.py:2186-2211）
		if desc := getString(plan, "description", ""); desc != "" {
			for _, part := range strings.Split(desc, ",") {
				part = strings.ToLower(strings.TrimSpace(part))
				if part == "" {
					continue
				}
				hasCPU := strings.Contains(part, "cpu") || strings.Contains(part, "core") ||
					strings.Contains(part, "i7") || strings.Contains(part, "i9") ||
					strings.Contains(part, "xeon") || strings.Contains(part, "epyc") || strings.Contains(part, "ryzen")
				if serverInfo.CPU == "N/A" && hasCPU {
					serverInfo.CPU = part
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

		// 解析方法 5: 从 plan.pricing.configurations 提取（1:1 对应 app.py:2213-2242）
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

		result = append(result, serverInfo)
	}

	return result
}

// parseBandwidthValue 与 Python 中带宽解析逻辑等价（1:1 对应 app.py:1914-1976）
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
	cpuKeywords := []string{"i7-", "i9-", "i5-", "xeon", "epyc", "ryzen"}
	for _, name := range []string{displayName, invoiceName, description} {
		if name == "" {
			continue
		}
		lcName := strings.ToLower(name)
		for _, kw := range cpuKeywords {
			if strings.Contains(lcName, kw) {
				pos := strings.Index(lcName, kw)
				end := pos + 30
				if end > len(name) {
					end = len(name)
				}
				cpuInfo := strings.TrimSpace(strings.Split(name[pos:end], ",")[0])
				if cpuInfo != "" {
					info.CPU = cpuInfo
					state.Logger.Info(fmt.Sprintf("从关键词中提取CPU型号: %s 给 %s", cpuInfo, info.PlanCode), "")
					return
				}
			}
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

// PassthroughAvailability 用 SDK 直接传请求（用于 sniper 监控）
func PassthroughAvailability(client *ovhsdk.Client, planCode string) ([]map[string]interface{}, error) {
	var out []map[string]interface{}
	q := url.Values{}
	q.Set("planCode", planCode)
	err := client.Get("/dedicated/server/datacenter/availabilities?"+q.Encode(), &out)
	return out, err
}

// 兼容 OVH 调用辅助：使 *ovhsdk.Client 通过返回值导出
var _ = ovh.RegionForDC
