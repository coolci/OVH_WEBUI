package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// ovhClientFor 从请求 ?account=xxx 取账户 ID 拿对应 OVH client;
// 空时(没传 ?account)走默认账户;凭据缺失返回 error,调用方按原 Client() 错误流程处理。
// 大部分 handler 都是 `state.OVH.Client()` 模式,这个 helper 是 1:1 替换,
// 把单账户改成多账户路由,语义最小化变化。
func ovhClientFor(state *app.State, c *gin.Context) (*ovhsdk.Client, error) {
	return state.OVH.ClientFor(c.Query("account"))
}

// ovhAccountFor 从请求 ?account=xxx 取账户实体(给需要原始凭据/endpoint 的 raw HTTP 调用用)。
// 空 → 默认账户;不存在 → ok=false。
func ovhAccountFor(state *app.State, c *gin.Context) (types.OVHAccount, bool) {
	return state.FindAccount(c.Query("account"))
}

// ovhAPIBaseURL 把 endpoint(ovh-eu / ovh-us / ovh-ca)映射成 OVH REST API base URL
func ovhAPIBaseURL(endpoint string) string {
	switch endpoint {
	case "ovh-us":
		return "https://api.us.ovhcloud.com"
	case "ovh-ca":
		return "https://ca.api.ovh.com"
	default:
		return "https://eu.api.ovh.com"
	}
}

// parallelGetDetails 通用并发 GET helper。对 keys[i] 用 pathFn(keys[i]) 拼出路径，
// 并发拉到 detail。最多 concurrency 个并发。结果按索引对齐，失败位 nil。
// 这是 1:1 串行 for 循环的并发替代版本，仅把网络 IO 并发化。
func parallelGetDetails(client *ovhsdk.Client, keys []interface{}, pathFn func(interface{}) string, concurrency int) []map[string]interface{} {
	if concurrency <= 0 {
		concurrency = 10
	}
	results := make([]map[string]interface{}, len(keys))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, k := range keys {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, key interface{}) {
			defer wg.Done()
			defer func() { <-sem }()
			var d map[string]interface{}
			if err := client.Get(pathFn(key), &d); err == nil {
				results[idx] = d
			}
		}(i, k)
	}
	wg.Wait()
	return results
}

// parallelGetStrings string 版本简化调用
func parallelGetStringKeys(client *ovhsdk.Client, keys []string, pathFn func(string) string, concurrency int) []map[string]interface{} {
	if concurrency <= 0 {
		concurrency = 10
	}
	results := make([]map[string]interface{}, len(keys))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, k := range keys {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, key string) {
			defer wg.Done()
			defer func() { <-sem }()
			var d map[string]interface{}
			if err := client.Get(pathFn(key), &d); err == nil {
				results[idx] = d
			}
		}(i, k)
	}
	wg.Wait()
	return results
}

// defaultZero 数字字段缺失时返回 0 而不是 null（OVH 偶尔不返回某些字段）
func defaultZero(v interface{}) interface{} {
	if v == nil {
		return 0
	}
	return v
}

// defaultObj 字典字段缺失时返回 {} 而不是 null
func defaultObj(v interface{}) interface{} {
	if v == nil {
		return map[string]interface{}{}
	}
	return v
}

// defaultArr 数组字段缺失时返回 [] 而不是 null
func defaultArr(v interface{}) interface{} {
	if v == nil {
		return []interface{}{}
	}
	return v
}

// isNonEmptyStorage 对应 Python "if data.get('storageConfig'):" 的 falsy 语义
// 空数组 / 空字典 / nil / false / 0 都视为"未提供自定义 storage"
func isNonEmptyStorage(v interface{}) bool {
	if v == nil {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case []interface{}:
		return len(x) > 0
	case map[string]interface{}:
		return len(x) > 0
	case string:
		return x != ""
	}
	return true
}

func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// idToString 把 OVH 返回的 ID（可能是 string 或数字）转成字符串
// 用于 /me/refund、/me/bill、/me/task/* 等返回数组里既可能是 ["xx","yy"] 也可能是 [1,2] 的端点
func idToString(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case json.Number:
		return x.String()
	default:
		return fmt.Sprintf("%v", x)
	}
}

// idToInt64 用于订单 ID 倒序排序
func idToInt64(v interface{}) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	default:
		n, _ := strconv.ParseInt(fmt.Sprintf("%v", x), 10, 64)
		return n
	}
}
