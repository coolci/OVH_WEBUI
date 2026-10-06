package handlers

import (
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// mitigationErrorFields 保留上游诊断信息，区分 IP 对象读取失败与已知缓解状态。
// 列表里有该 IP 不代表 /ip/{ip}/mitigation 能读取；404 也不代表服务器下线或没有防护。
func mitigationErrorFields(err error) gin.H {
	fields := gin.H{"error": ovh.Explain(err), "detail": err.Error()}
	var apiErr *ovhsdk.APIError
	if errors.As(err, &apiErr) {
		fields["upstreamStatus"] = apiErr.Code
		if apiErr.QueryID != "" {
			fields["queryId"] = apiErr.QueryID
		}
		message := strings.ToLower(apiErr.Message)
		if apiErr.Code == http.StatusNotFound &&
			strings.Contains(message, "requested object (ip =") && strings.Contains(message, "does not exist") {
			fields["errorCode"] = "IP_OBJECT_NOT_FOUND"
			fields["error"] = "OVH 无法读取该 IP 对象，当前 DDoS 缓解状态未知。请在 OVH 控制台核对该 IP；持续失败请携带查询号联系 OVH 支持。"
		}
	}
	return fields
}

// GetMitigation GET /api/server-control/:service_name/mitigation
//
// 列服务器所有 IP 的 DDoS 缓解状态。
// OVH 的 /ip/{ip}/mitigation 端点要求 {ip} 是 IP 块(/32 用 %2F 转义),
// 使用 /dedicated/server/{svc}/ips 返回的原始块名，不猜测掩码；详情不可读时保留未知状态。
//
// 返回结构:
//
//	ips: [{ ipBlock, mitigations: [{ ipOnMitigation, state, auto, permanent }] }]
func GetMitigation(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var ipBlocks []string
		if err := client.Get("/dedicated/server/"+svc+"/ips", &ipBlocks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		type ipResult struct {
			block       string
			mitigations []map[string]interface{}
			note        string
			err         error
		}
		results := make([]ipResult, len(ipBlocks))
		sem := make(chan struct{}, 8)
		// 详情请求单独限流：外层 goroutine 不参与这个信号量，两层不会互相饿死。
		detailSem := make(chan struct{}, 16)
		var wg sync.WaitGroup
		for i, blk := range ipBlocks {
			// /ip/{ip}/mitigation 返回 ipv4[]，ip.MitigationIp.ipOnMitigation 也是 ipv4：
			// 该查询接口仅支持 IPv4，跳过 IPv6；接口的限制不能用于判断 IPv6 防护状态。
			if strings.Contains(blk, ":") {
				results[i] = ipResult{
					block: blk,
					note:  "此 IPv4 缓解接口不支持 IPv6，不能据此判断 IPv6 防护状态",
				}
				continue
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, ipBlock string) {
				defer wg.Done()
				defer func() { <-sem }()
				encoded := strings.ReplaceAll(ipBlock, "/", "%2F")
				// 1) 列出该 block 下处于 mitigation 的具体 IP
				var ips []string
				if err := client.Get("/ip/"+encoded+"/mitigation", &ips); err != nil {
					results[idx] = ipResult{block: ipBlock, err: err}
					return
				}
				// 2) 并发拉每个 IP 的详情。失败的 IP 也必须留在结果里：
				// 直接跳过会让「正在被缓解」的 IP 从页面上凭空消失，前端据此显示
				// 「无永久缓解」并诱导用户重复点开启，属于把瞬时错误伪装成业务状态。
				details := make([]map[string]interface{}, len(ips))
				var dwg sync.WaitGroup
				for j, ip := range ips {
					dwg.Add(1)
					detailSem <- struct{}{}
					go func(jdx int, ipOnMitigation string) {
						defer dwg.Done()
						defer func() { <-detailSem }()
						var d map[string]interface{}
						if err := client.Get("/ip/"+encoded+"/mitigation/"+ipOnMitigation, &d); err != nil {
							state.Logger.Warn("[Mitigation] 获取 "+ipOnMitigation+" 缓解详情失败: "+err.Error(), "server_control")
							// 占位要带齐前端会读的字段,否则 state/auto/permanent 是 undefined,
							// 界面渲染出一个空白 Chip,看不出这行是"拉取失败"
							details[jdx] = map[string]interface{}{
								"ipOnMitigation": ipOnMitigation,
								"state":          "unknown",
								"auto":           false,
								"permanent":      false,
								"_detailError":   err.Error(),
							}
							for key, value := range mitigationErrorFields(err) {
								details[jdx][key] = value
							}
							return
						}
						details[jdx] = d
					}(j, ip)
				}
				dwg.Wait()
				results[idx] = ipResult{block: ipBlock, mitigations: details}
			}(i, blk)
		}
		wg.Wait()

		list := []gin.H{}
		for _, r := range results {
			row := gin.H{"ipBlock": r.block, "mitigations": r.mitigations}
			if r.err != nil {
				for key, value := range mitigationErrorFields(r.err) {
					row[key] = value
				}
				state.Logger.Warn("[Mitigation] 获取 "+r.block+" 缓解列表失败: "+r.err.Error(), "server_control")
			}
			if r.note != "" {
				row["note"] = r.note
			}
			if r.mitigations == nil {
				row["mitigations"] = []interface{}{}
			}
			list = append(list, row)
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "ips": list})
	}
}

// 永久缓解已于 2025 年退役。POST /ip/{ip}/mitigation 虽然仍存在，但手动创建
// 不再启用 permanent 模式，不能将创建对象成功报告成“已开启永久防护”。
// 官方依据: https://github.com/ovh/infrastructure-roadmap/issues/282 及现行 /ip.json。
func permanentMitigationRemoved(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"success": false,
		"error":   "OVH 已停用 IP 永久缓解模式。自动 DDoS 防护无需手动开启，当前页面仅查询缓解状态。",
		"code":    "PERMANENT_MITIGATION_REMOVED",
	})
}

// EnableMitigation 保留本地路由兼容，明确拒绝已退役的永久模式操作。
func EnableMitigation(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		permanentMitigationRemoved(c)
	}
}

// DisableMitigation 不删除自动缓解对象，避免把只读状态查询混成防护开关。
func DisableMitigation(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		permanentMitigationRemoved(c)
	}
}
