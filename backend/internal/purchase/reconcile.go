package purchase

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/ovh"
)

// reconcileScanCap 对账只看最新的若干笔。订单号随时间递增，
// 结账超时后要找的是刚刚生成的那一笔，不必扫完整本订单史。
const reconcileScanCap = 25

// reconcileDetailCap 一笔订单的明细通常只有几行（主机 + 安装费）。
// 封顶是为了在异常大单上不会把对账拖成一次目录级拉取。
const reconcileDetailCap = 12

// ReconcileOptions 对账增强选项
type ReconcileOptions struct {
	Datacenter       string
	ExcludedOrderIDs map[string]bool
}

// ReconcileRecentOrder 兼容旧调用入口
func ReconcileRecentOrder(client *ovhsdk.Client, endpoint, planCode string, since time.Time) (orderID, orderURL string, err error) {
	return ReconcileRecentOrderEx(client, endpoint, planCode, since, ReconcileOptions{})
}

// ReconcileRecentOrderEx 在 checkout 超时/5xx 之后查官方订单，确认这轮有没有已经建成。
// 增加 ExcludedOrderIDs 排除历史已认领订单，以及 Datacenter 冲突检测。
func ReconcileRecentOrderEx(client *ovhsdk.Client, endpoint, planCode string, since time.Time, opts ReconcileOptions) (orderID, orderURL string, err error) {
	planCode = strings.TrimSpace(planCode)
	if client == nil || planCode == "" {
		return "", "", fmt.Errorf("reconcile: missing client or planCode")
	}
	ids, objects, err := listRecentOrders(client, endpoint, since)
	if err != nil {
		return "", "", err
	}
	// 兼容直接返回对象列表的 mock 或端点 (e.g. [{"orderId":123,"url":"..."}])
	for _, obj := range objects {
		id := numconv.ToString(obj["orderId"])
		if id != "" {
			if opts.ExcludedOrderIDs != nil && opts.ExcludedOrderIDs[id] {
				continue
			}
			link, _ := obj["url"].(string)
			link = strings.TrimSpace(link)
			if link == "" {
				link = ovh.ManagerOrderURL(endpoint, id)
			}
			return id, link, nil
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
	if len(ids) > reconcileScanCap {
		ids = ids[:reconcileScanCap]
	}
	cutoff := since.Add(-2 * time.Minute)
	uncertain := false
	for _, id := range ids {
		idStr := strconv.FormatInt(id, 10)
		if opts.ExcludedOrderIDs != nil && opts.ExcludedOrderIDs[idStr] {
			continue
		}
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
				if opts.Datacenter != "" && orderDetailContradictsDC(detail, opts.Datacenter) {
					continue
				}
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
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

func orderDetailContradictsDC(detail map[string]interface{}, targetDC string) bool {
	target := strings.ToLower(strings.TrimSpace(targetDC))
	if target == "" {
		return false
	}
	desc, _ := detail["description"].(string)
	domain, _ := detail["domain"].(string)
	blob := strings.ToLower(desc + "\n" + domain)

	knownDCs := []string{"rbx", "gra", "sbg", "bhs", "waw", "fra", "lon", "syd", "sgp", "vin", "hil"}
	// 必须按完整机房记号匹配,不能用子串:"graphics" 含 gra、"along" 含 lon、
	// "saving" 含 vin、"frame" 含 fra。子串匹配会把一张真实建成的订单判成"别的机房",
	// 对账就认不出它,结账超时后的重试会再买一台。
	if !containsDCToken(blob, target) {
		for _, dc := range knownDCs {
			if dc != target && containsDCToken(blob, dc) {
				return true
			}
		}
	}
	return false
}

// containsDCToken 判断 blob 里是否出现独立的机房记号(前后不能紧挨字母;
// 允许数字后缀,如 gra3 / rbx8)。blob 需已转小写。
func containsDCToken(blob, dc string) bool {
	for start := 0; ; {
		i := strings.Index(blob[start:], dc)
		if i < 0 {
			return false
		}
		i += start
		end := i + len(dc)
		beforeOK := i == 0 || !isASCIILetter(blob[i-1])
		afterOK := end == len(blob) || !isASCIILetter(blob[end])
		if beforeOK && afterOK {
			return true
		}
		start = i + 1
	}
}

func isASCIILetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }

func listRecentOrders(client *ovhsdk.Client, endpoint string, since time.Time) (ids []int64, objects []map[string]interface{}, err error) {
	// US 的 /me/order 没有查询参数。其它区用 date.from，被 400 拒绝时再降级成全量，
	// 避免 endpoint 标错时整段对账失效。429/5xx 不降级，避免再打一枪。
	queryPath := "/me/order"
	if ovh.EndpointRegion(endpoint) != "US" {
		from := since.Add(-2 * time.Minute).UTC().Format("2006-01-02T15:04:05Z07:00")
		queryPath = "/me/order?date.from=" + url.QueryEscape(from)
	}

	var rawList []json.RawMessage
	err = client.Get(queryPath, &rawList)
	if err != nil && queryPath != "/me/order" {
		var apiErr *ovhsdk.APIError
		if errors.As(err, &apiErr) && apiErr.Code == http.StatusBadRequest {
			err = client.Get("/me/order", &rawList)
		}
	}
	if err != nil {
		return nil, nil, err
	}

	for _, item := range rawList {
		trimmed := strings.TrimSpace(string(item))
		if strings.HasPrefix(trimmed, "{") {
			var obj map[string]interface{}
			if err := json.Unmarshal(item, &obj); err == nil {
				objects = append(objects, obj)
			}
		} else {
			var id int64
			if err := json.Unmarshal(item, &id); err == nil {
				ids = append(ids, id)
			}
		}
	}
	return ids, objects, nil
}

func orderDetailMatchesPlan(detail map[string]interface{}, planCode string) bool {
	pc := strings.ToLower(strings.TrimSpace(planCode))
	if len(pc) < 3 {
		return false
	}
	desc, _ := detail["description"].(string)
	domain, _ := detail["domain"].(string)
	blob := strings.ToLower(desc + "\n" + domain)
	// Model names may share prefixes or contain punctuation; match the whole
	// model token rather than claiming a neighbouring model's order.
	return regexp.MustCompile(`(^|[^a-z0-9_.-])` + regexp.QuoteMeta(pc) + `($|[^a-z0-9_.-])`).MatchString(blob)
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
