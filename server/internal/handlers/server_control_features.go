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

// featureOVHErr 拆出 OVH 错误里的 HTTP 状态码和原始 message。
func featureOVHErr(err error) (int, string) {
	var apiErr *ovhsdk.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code, apiErr.Message
	}
	return 0, err.Error()
}

// featureIsFeatureMissing 判断一个 404/"does not exist" 到底是"该功能未开通"还是"服务器压根不在这个账户下"。
func featureIsFeatureMissing(client *ovhsdk.Client, svc string) bool {
	srvPath, err := buildOVHPath("dedicated", "server", svc)
	if err != nil {
		return false
	}
	var d map[string]interface{}
	if err := client.Get(srvPath, &d); err != nil {
		code, _ := featureOVHErr(err)
		if code == http.StatusNotFound || code == http.StatusForbidden {
			return false
		}
		return true
	}
	return true
}

// featureBackupFTPUnavailable 在 US 区直接拦掉整套备份FTP 请求。
// 依据：官方 schema 里 US endpoint 的 /dedicated/server 只有 98 条路径，backupFTP 五条一条都没有
// （EU 与 CA 都是 107 条、五条齐全，已用 live api.us.ovhcloud.com/1.0/dedicated/server.json
// 与 eu./ca. 三站并排复核；US 侧同类能力只有 backupCloud，那三条端点三区都在，不需要门控）。
// 不拦的话请求会拿到一句 "Got an invalid (or empty) URL" 的 500，对用户毫无意义。
//
// 判大区走 srvRegionFor(见 server_control_basic.go)而不是比 acc.Endpoint 字符串：
// endpoint 还有 kimsufi-* / soyoustart-* 别名，散装比较漏一种写法就等于门控失效。
//
// 状态码按方法分开，是为了对上前端 use-server-control.ts 的两套读法：
// GET 走 useServerBackupFtp，它是"状态码优先"——任何 404 都会被当成"未激活"并丢掉整个 body，
// 于是 US 用户会看到一个点了必然失败的"激活 Backup FTP"按钮；而 200 + success:false 正好命中
// 该 hook 的 notAvailable 分支，页面显示"此服务器无 Backup FTP + 原因"，无需前端配合。
// 写操作（POST/DELETE）走 mutation，必须以非 2xx 结束才会走 catch 弹错误提示，
// 否则会假装"激活请求已发送"，所以这里返回 404，body 里的 error 前端会读。
func featureBackupFTPUnavailable(state *app.State, c *gin.Context) bool {
	region := srvRegionFor(state, c)
	if region != srvRegionUS {
		return false
	}
	state.Logger.Info("账户所在大区 "+region+" 不提供备份FTP功能，已拦截请求", "server_control")
	payload := gin.H{
		"success": false,
		"error":   "当前账户所在区域（US）不提供备份FTP功能，请改用云备份（Backup Cloud）", "code": "E192FDC9B",
		"notAvailable": true,
		"unsupported":  true,
		"region":       region,
	}
	if c.Request.Method == http.MethodGet {
		c.JSON(http.StatusOK, payload)
	} else {
		c.JSON(http.StatusNotFound, payload)
	}
	return true
}

// featureBackupFTPACLPath 安全构造 ACL 详情路径：逐段校验拒绝 "." / ".." 并编码 "/" -> "%2F"。
func featureBackupFTPACLPath(svc, ipBlock string) (string, error) {
	if _, ok := validateIPOrCIDR(ipBlock); !ok {
		return "", errUnsafeOVHSegment
	}
	return buildOVHPath("dedicated", "server", svc, "features", "backupFTP", "access", ipBlock)
}

// featureParallelACLDetails 并发拉每个 ipBlock 的 ACL 详情，失败位把真实错误留下来。
func featureParallelACLDetails(client *ovhsdk.Client, svc string, blocks []string, concurrency int) ([]map[string]interface{}, []error) {
	if concurrency <= 0 {
		concurrency = 10
	}
	details := make([]map[string]interface{}, len(blocks))
	errs := make([]error, len(blocks))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, b := range blocks {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, block string) {
			defer wg.Done()
			defer func() { <-sem }()
			aclPath, pErr := featureBackupFTPACLPath(svc, block)
			if pErr != nil {
				errs[idx] = pErr
				return
			}
			var d map[string]interface{}
			client, err := ovh.IsolateClient(client)
			if err != nil {
				errs[idx] = err
				return
			}
			if err := client.Get(aclPath, &d); err != nil {
				errs[idx] = err
				return
			}
			details[idx] = d
		}(i, b)
	}
	wg.Wait()
	return details, errs
}

// Burst GET
func GetBurst(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "burst")
		if !ok {
			return
		}
		var burst map[string]interface{}
		if err := client.Get(p, &burst); err != nil {
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "does not exist") || strings.Contains(lower, "not exist") {
				state.Logger.Info("服务器 "+svc+" 不支持突发带宽功能", "server_control")
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "该服务器不支持突发带宽功能", "code": "EA43E3E3C", "notAvailable": true})
				return
			}
			respondOVHError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "burst": burst})
	}
}

// Burst PUT
func UpdateBurst(state *app.State) gin.HandlerFunc {
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
		var body struct {
			Status string `json:"status"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Status == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少status参数", "code": "E8C3A474D"})
			return
		}
		switch body.Status {
		case "active", "inactive", "inactiveLocked":
		default:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "status 参数无效，只能是 active / inactive / inactiveLocked", "code": "E5244ABC0"})
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "burst")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Put(p, map[string]interface{}{
			"status": body.Status,
		}, &result); err != nil {
			status := http.StatusInternalServerError
			if code, _ := featureOVHErr(err); code == http.StatusBadRequest {
				status = http.StatusBadRequest
			}
			state.Logger.Error("更新服务器 "+svc+" 突发带宽状态失败: "+err.Error(), "server_control")
			c.JSON(status, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("更新服务器 "+svc+" 突发带宽状态为: "+body.Status, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "突发带宽状态已更新", "code": "EA72B810D", "result": result})
	}
}

// Firewall GET
func GetFirewall(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "firewall")
		if !ok {
			return
		}
		var fw map[string]interface{}
		if err := client.Get(p, &fw); err != nil {
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "does not exist") || strings.Contains(lower, "not exist") {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "该服务器不支持防火墙功能", "code": "E38DF225C", "notAvailable": true})
				return
			}
			respondOVHError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "firewall": fw})
	}
}

// Firewall PUT
func UpdateFirewall(state *app.State) gin.HandlerFunc {
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
		var body struct {
			Enabled *bool `json:"enabled"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Enabled == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少enabled参数", "code": "E3D2DC322"})
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "firewall")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Put(p, map[string]interface{}{
			"enabled": *body.Enabled,
		}, &result); err != nil {
			respondOVHError(c, err)
			return
		}
		text := "启用"
		if !*body.Enabled {
			text = "禁用"
		}
		state.Logger.Info(text+"服务器 "+svc+" 防火墙", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "防火墙已" + text, "result": result})
	}
}
