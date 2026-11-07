package vps

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// 一轮检查里,同一子公司 + 同一查询账户的所有 VPS 订阅用一次
// GET /vps/order/rule/datacenter/aggregated(免登录,planCodes 逗号分隔)查完,
// 代替"每个订阅一次 /vps/order/rule/datacenter + 间隔 1 秒"。
// 实测(2026-11,IE)本地盘条目与逐个查询的结果逐字相同;远程盘(remoteStorage=true)
// 条目逐个查询接口不返回,这里同样丢弃,保持口径一致。
// 带系统过滤(os)的订阅聚合接口不支持,仍走逐个查询。

// aggregatedBatchSize 单次请求最多带几个型号,避免 URL 过长
const aggregatedBatchSize = 25

type aggregatedKey struct{ subsidiary, accountID, planCode string }

// prefetchAggregated 返回 (子公司, 账户, 型号) → 与 checkVPSDCAvailabilityOS 同形的 {"datacenters": [...]}。
// 任何一批失败都只是少了预取结果,调用方对缺失的型号照旧逐个查询。
func prefetchAggregated(state *app.State, subs []types.VPSSubscription) map[aggregatedKey]map[string]interface{} {
	type group struct{ subsidiary, accountID string }
	plans := map[group]map[string]bool{}
	for _, s := range subs {
		if strings.TrimSpace(s.OS) != "" {
			continue
		}
		sub := NormalizeSubsidiary(s.OvhSubsidiary)
		if sub == "" {
			sub = DefaultSubsidiary(state, s.AutoOrderAccountID)
		}
		g := group{sub, s.AutoOrderAccountID}
		if plans[g] == nil {
			plans[g] = map[string]bool{}
		}
		plans[g][strings.TrimSpace(s.PlanCode)] = true
	}
	out := map[aggregatedKey]map[string]interface{}{}
	for g, set := range plans {
		codes := make([]string, 0, len(set))
		for c := range set {
			if c != "" {
				codes = append(codes, c)
			}
		}
		sort.Strings(codes)
		for start := 0; start < len(codes); start += aggregatedBatchSize {
			end := start + aggregatedBatchSize
			if end > len(codes) {
				end = len(codes)
			}
			models, err := fetchAggregated(state, g.subsidiary, g.accountID, codes[start:end])
			if err != nil {
				state.Logger.Warn(fmt.Sprintf("VPS 聚合库存查询失败(子公司 %s,改为逐个查询): %s", g.subsidiary, err.Error()), "vps_monitor")
				continue
			}
			for plan, data := range models {
				out[aggregatedKey{g.subsidiary, g.accountID, plan}] = data
			}
		}
	}
	return out
}

func fetchAggregated(state *app.State, subsidiary, accountID string, planCodes []string) (map[string]map[string]interface{}, error) {
	q := url.Values{}
	q.Set("ovhSubsidiary", subsidiary)
	q.Set("planCodes", strings.Join(planCodes, ","))
	req, _ := http.NewRequest(http.MethodGet, vpsAPIBaseURL(subsidiary)+"/v1/vps/order/rule/datacenter/aggregated?"+q.Encode(), nil)
	req.Header.Set("accept", "application/json")
	client, err := availabilityHTTPClient(state, accountID, 15*time.Second)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, bodySnippet(body))
	}
	var payload struct {
		Models []struct {
			PlanCode    string                   `json:"planCode"`
			Datacenters []map[string]interface{} `json:"datacenters"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("返回的不是合法 JSON: %s", bodySnippet(body))
	}
	out := map[string]map[string]interface{}{}
	for _, m := range payload.Models {
		dcs := make([]interface{}, 0, len(m.Datacenters))
		for _, dc := range m.Datacenters {
			if remote, _ := dc["remoteStorage"].(bool); remote {
				continue
			}
			dcs = append(dcs, dc)
		}
		out[m.PlanCode] = map[string]interface{}{"datacenters": dcs}
	}
	return out, nil
}
