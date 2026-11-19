package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// Orderable
func GetOrderableBandwidth(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "orderable", "bandwidth")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "orderable": d})
	}
}

func GetOrderableTraffic(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "orderable", "traffic")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "orderable": d})
	}
}

func GetOrderableIP(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "orderable", "ip")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "orderable": d})
	}
}

// Options
func GetServerOptions(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "option")
		if !ok {
			return
		}
		var opts []string
		if err := client.Get(p, &opts); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		details, detailErrs := miscParallelGetStringKeys(client, opts, func(o string) string {
			return safePathFn("dedicated", "server", svc, "option", o)
		}, 10)
		list, failed := miscMergeDetails(miscKeysAsIface(opts), "option", details, detailErrs)
		if failed > 0 {
			state.Logger.Warn(fmt.Sprintf("服务器 %s 选项详情有 %d/%d 项拉取失败", svc, failed, len(opts)), "server_control")
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "options": list, "partial": failed > 0, "failedCount": failed})
	}
}

// IP specs
func GetIPSpecs(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "specifications", "ip")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "ipSpecs": d})
	}
}

func GetIPCanBeMovedTo(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		// schema: ip 是 query 必填(类型 ipBlock,可以带掩码)
		ip := strings.TrimSpace(c.Query("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少ip参数", "code": "EB7965AF4"})
			return
		}
		if _, ok := validateIPOrCIDR(ip); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ip 参数格式不合法", "code": "INVALID_IP"})
			return
		}
		checkPath, ok := ovhPathQueryOrAbort(c, url.Values{"ip": {ip}}, "dedicated", "server", svc, "ipCanBeMovedTo")
		if !ok {
			return
		}
		if err := client.Get(checkPath, nil); err != nil {
			state.Logger.Warn("检查 IP "+ip+" 能否迁移到服务器 "+svc+" 失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err), "ip": ip})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "canBeMoved": true, "ip": ip})
	}
}

func GetIPCountryAvailable(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "ipCountryAvailable")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "countries": d})
	}
}

func MoveIP(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		// /ipMove 的语义是"把这个 IP 搬到 URL 里的这台服务器",目标就是 svc 本身。
		// 防呆护栏:若旧调用方误把源实例放在路由 :service_name、目标实例放在请求体 to 中,
		// 必须立即 400 拦截,绝不把故障转移 IP 反向迁回源机器！
		var body struct {
			IP string `json:"ip"`
			To string `json:"to"`
		}
		_ = c.ShouldBindJSON(&body)
		body.IP = strings.TrimSpace(body.IP)
		body.To = strings.TrimSpace(body.To)
		if body.IP == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少ip参数", "code": "EB7965AF4"})
			return
		}
		if _, ok := validateIPOrCIDR(body.IP); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ip 参数格式不合法", "code": "INVALID_IP"})
			return
		}
		if body.To != "" && !strings.EqualFold(body.To, svc) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "路由服务名与请求体目标服务器(to)冲突：POST /server-control/{service_name}/ip/move 中的 {service_name} 必须为迁入目标服务器",
				"code":    "MOVE_IP_TARGET_MISMATCH",
			})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		movePath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "ipMove")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Post(movePath, map[string]interface{}{
			"ip": body.IP,
		}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("IP迁移任务已创建: "+body.IP+" -> "+svc, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "IP迁移任务已创建", "code": "E26999A49", "result": result})
	}
}
