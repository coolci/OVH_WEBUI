package purchase

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/ovh"
)

// reconcileScanCap 对账只看最新的若干笔。订单号随时间递增，
// 结账超时后要找的是刚刚生成的那一笔，不必扫完整本订单史。
const reconcileScanCap = 25

// reconcileDetailCap 一笔订单的明细通常只有几行（主机 + 安装费）。
// 封顶是为了在异常大单上不会把对账拖成一次目录级拉取。
const reconcileDetailCap = 12

// ReconcileRecentOrder 在 checkout 超时/5xx 之后查官方订单，确认这轮有没有已经建成。
//
// 官方 GET /me/order（eu.api.ovh.com / ca.api.ovh.com / api.us.ovhcloud.com 的 /me.json，
// 以及本仓库 schemacheck 基线）:
//   - 响应是 long[]（订单号），不是带 orderId/url 的对象数组
//   - EU/CA 的查询参数是 date.from / date.to（datetime）；没有 dateFrom，也没有 planCode
//   - US 不声明任何查询参数，带了参数可能直接 400
//
// 旧实现对着 []map 解 long[]，再附上并不存在的 dateFrom 和 planCode。
// 解不开或被 400 时一律当成“没建单”。调用方随后暂停任务，用户如果恢复重试，
// 已经建成的那一笔会被再买一次。
//
// 命中条件：订单 date 不早于 since 前 2 分钟（盖住时钟差），且任一明细的
// description 或 domain 包含 planCode。返回的 url 优先用订单自带的付款链接
// （只进本地历史，通知另走控制台深链）；没有时回退到控制台深链。
func ReconcileRecentOrder(client *ovhsdk.Client, endpoint, planCode string, since time.Time) (orderID, orderURL string, err error) {
	planCode = strings.TrimSpace(planCode)
	if client == nil || planCode == "" {
		return "", "", fmt.Errorf("reconcile: missing client or planCode")
	}
	ids, err := listRecentOrderIDs(client, endpoint, since)
	if err != nil {
		return "", "", err
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
	if len(ids) > reconcileScanCap {
		ids = ids[:reconcileScanCap]
	}
	cutoff := since.Add(-2 * time.Minute)
	uncertain := false
	for _, id := range ids {
		var order map[string]interface{}
		if err := client.Get(fmt.Sprintf("/me/order/%d", id), &order); err != nil {
			uncertain = true
			continue
		}
		dateStr, _ := order["date"].(string)
		when, ok := parseOrderTime(dateStr)
		if !ok || when.Before(cutoff) {
			continue
		}
		var detailIDs []int64
		if err := client.Get(fmt.Sprintf("/me/order/%d/details", id), &detailIDs); err != nil {
			uncertain = true
			continue
		}
		if len(detailIDs) > reconcileDetailCap {
			detailIDs = detailIDs[:reconcileDetailCap]
		}
		matched := false
		for _, did := range detailIDs {
			var detail map[string]interface{}
			if err := client.Get(fmt.Sprintf("/me/order/%d/details/%d", id, did), &detail); err != nil {
				uncertain = true
				continue
			}
			if orderDetailMatchesPlan(detail, planCode) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		idStr := strconv.FormatInt(id, 10)
		link, _ := order["url"].(string)
		link = strings.TrimSpace(link)
		if link == "" {
			link = ovh.ManagerOrderURL(endpoint, idStr)
		}
		return idStr, link, nil
	}
	if uncertain {
		return "", "", fmt.Errorf("reconcile: order list fetched but detail lookup failed")
	}
	return "", "", nil
}

func listRecentOrderIDs(client *ovhsdk.Client, endpoint string, since time.Time) ([]int64, error) {
	// US 的 /me/order 没有查询参数。其它区用 date.from，被 400 拒绝时再降级成全量，
	// 避免 endpoint 标错时整段对账失效。429/5xx 不降级，避免再打一枪。
	if ovh.EndpointRegion(endpoint) != "US" {
		from := since.Add(-2 * time.Minute).UTC().Format("2006-01-02T15:04:05Z07:00")
		var ids []int64
		err := client.Get("/me/order?date.from="+url.QueryEscape(from), &ids)
		if err == nil {
			return ids, nil
		}
		var apiErr *ovhsdk.APIError
		if !errors.As(err, &apiErr) || apiErr.Code != http.StatusBadRequest {
			return nil, err
		}
	}
	var ids []int64
	if err := client.Get("/me/order", &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func orderDetailMatchesPlan(detail map[string]interface{}, planCode string) bool {
	pc := strings.ToLower(strings.TrimSpace(planCode))
	if len(pc) < 3 {
		return false
	}
	desc, _ := detail["description"].(string)
	domain, _ := detail["domain"].(string)
	blob := strings.ToLower(desc + "\n" + domain)
	return strings.Contains(blob, pc)
}

func parseOrderTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
