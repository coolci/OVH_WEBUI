package handlers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

// ── 统一 OVH 路径、查询与参数安全门禁 ─────────────────────────────────────
//
// 所有发往 OVH SDK 的动态路径段必须逐段校验非空、拒绝 "." / ".." 与控制字符，
// 并经 url.PathEscape 编码；所有查询参数必须通过 url.Values.Encode() 编码，
// 严禁手拼 "?key=" + val 或将未转义路由参数拼入上游路径。

var errUnsafeOVHSegment = errors.New("OVH 路径段包含非法或越界字符")

// validateOVHSegment 校验单个原始路径段：非空、非 "." / ".."、无控制字符、解码后亦不为遍历段。
func validateOVHSegment(seg string) error {
	if strings.TrimSpace(seg) == "" || seg == "." || seg == ".." {
		return errUnsafeOVHSegment
	}
	for i := 0; i < len(seg); i++ {
		b := seg[i]
		if b < 0x20 || b == 0x7f || b == '\\' {
			return errUnsafeOVHSegment
		}
	}
	if unesc, err := url.PathUnescape(seg); err == nil {
		if unesc == "" || unesc == "." || unesc == ".." ||
			strings.Contains(unesc, "../") || strings.Contains(unesc, "/..") ||
			strings.ContainsAny(unesc, "\x00\r\n\\") {
			return errUnsafeOVHSegment
		}
	}
	return nil
}

// buildOVHPath 逐段校验并执行 url.PathEscape 构造安全的 OVH API 路径（以 "/" 开头）。
func buildOVHPath(segments ...string) (string, error) {
	if len(segments) == 0 {
		return "", errUnsafeOVHSegment
	}
	escaped := make([]string, len(segments))
	for i, seg := range segments {
		if err := validateOVHSegment(seg); err != nil {
			return "", err
		}
		escaped[i] = url.PathEscape(seg)
	}
	return "/" + strings.Join(escaped, "/"), nil
}

// safePathFn 供并发详情拉取回调使用：非法路径段返回空字符串，由并发执行器拦截拒绝。
func safePathFn(segments ...string) string {
	p, err := buildOVHPath(segments...)
	if err != nil {
		return ""
	}
	return p
}

// buildOVHPathQuery 构造安全的 OVH API 路径并追加经 url.Values 编码的查询参数。
func buildOVHPathQuery(q url.Values, segments ...string) (string, error) {
	base, err := buildOVHPath(segments...)
	if err != nil {
		return "", err
	}
	if len(q) == 0 {
		return base, nil
	}
	return base + "?" + q.Encode(), nil
}

// ovhPathOrAbort 在 Gin handler 中安全构造 OVH 路径，失败时直接响应 400 并中止。
func ovhPathOrAbort(c *gin.Context, segments ...string) (string, bool) {
	p, err := buildOVHPath(segments...)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请求参数包含非法路径字符", "code": "INVALID_OVH_PATH"})
		return "", false
	}
	return p, true
}

// ovhPathQueryOrAbort 在 Gin handler 中安全构造带查询参数的 OVH 路径，失败时响应 400。
func ovhPathQueryOrAbort(c *gin.Context, q url.Values, segments ...string) (string, bool) {
	p, err := buildOVHPathQuery(q, segments...)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请求参数包含非法路径字符", "code": "INVALID_OVH_PATH"})
		return "", false
	}
	return p, true
}

// isValidServiceName 校验 OVH 服务名合法性（字母、数字、点、连字符、下划线，禁止连续点或首尾点，长度 <= 255）。
func isValidServiceName(svc string) bool {
	if svc == "" || len(svc) > maxServiceNameLen || svc == "." || svc == ".." ||
		strings.Contains(svc, "..") || strings.HasPrefix(svc, ".") || strings.HasSuffix(svc, ".") {
		return false
	}
	for _, r := range svc {
		ok := r == '.' || r == '-' || r == '_' ||
			(r >= '0' && r <= '9') ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z')
		if !ok {
			return false
		}
	}
	return true
}

// requireServiceName 在 handler 入口校验 :service_name（即使未挂载路由组中间件亦受保护）。
func requireServiceName(c *gin.Context) (string, bool) {
	svc := strings.TrimSpace(c.Param("service_name"))
	if !isValidServiceName(svc) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "服务名格式不合法，只允许字母、数字、点、连字符和下划线",
			"code":    "E9A400893",
		})
		return "", false
	}
	return svc, true
}

// requirePositiveIDParam 校验路径参数必须为正整数（如 :task_id, :intervention_id, :boot_id, :orderId）。
func requirePositiveIDParam(c *gin.Context, paramName, label, errCode string) (string, bool) {
	raw := strings.TrimSpace(c.Param(paramName))
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		if errCode == "" {
			errCode = "INVALID_ID_PARAM"
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   label + "格式不正确，必须是正整数",
			"code":    errCode,
		})
		return "", false
	}
	return strconv.FormatInt(id, 10), true
}

// requireSafeSingleSegment 校验单段标识参数（如 :domain, :vrack, :option），禁止斜杠、问号、井号、百分号与遍历符。
func requireSafeSingleSegment(c *gin.Context, paramName, label, errCode string) (string, bool) {
	val := strings.TrimSpace(c.Param(paramName))
	if err := validateOVHSegment(val); err != nil || strings.ContainsAny(val, "/?#%& ") {
		if errCode == "" {
			errCode = "INVALID_PATH_PARAM"
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   label + "格式不合法",
			"code":    errCode,
		})
		return "", false
	}
	return val, true
}

// validateIPOrCIDR 校验字符串必须是合法 IP 地址或 CIDR 网段（如 1.2.3.4 或 1.2.3.4/32）。
func validateIPOrCIDR(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" || strings.ContainsAny(s, "?#%\\ \t\r\n") {
		return "", false
	}
	if ip := net.ParseIP(s); ip != nil {
		return s, true
	}
	if _, _, err := net.ParseCIDR(s); err == nil {
		return s, true
	}
	return "", false
}

// validateSingleIP 校验字符串必须是合法的单个 IPv4/IPv6 地址。
func validateSingleIP(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" || strings.ContainsAny(s, "/?#%\\ \t\r\n") {
		return "", false
	}
	if ip := net.ParseIP(s); ip != nil {
		return s, true
	}
	return "", false
}

// accountCacheFingerprint 生成包含账户 ID、站点与当前凭据指纹的隔离缓存键。
// 当用户在「设置 → OVH 账户」轮换密钥、切换站点或删号重建后，指纹立即变化，
// 控制面缓存 0ms 自动失效，且多账户之间严格物理隔离。
func accountCacheFingerprint(acc types.OVHAccount) string {
	return strings.Join([]string{
		acc.ID,
		acc.Endpoint,
		acc.Zone,
		acc.AppKey,
		acc.AppSecret,
		acc.ConsumerKey,
	}, "|")
}

// 账户安全面板:列出 OVH 账户下的 API 凭据(哪个应用、何时创建、最近使用、到期时间、
// 权限规则、IP 限制)和本账户最近的 API 调用记录,并可吊销凭据。
// 用于发现泄露或遗忘的凭据。本程序自己正在用的那条凭据禁止吊销 —— 吊销就把自己锁在门外了。

const maxCredentialsListed = 60
const maxAPILogsListed = 30

type securityCredential struct {
	CredentialID  int64    `json:"credentialId"`
	ApplicationID int64    `json:"applicationId"`
	AppName       string   `json:"appName,omitempty"`
	AppDesc       string   `json:"appDescription,omitempty"`
	Status        string   `json:"status"`
	Creation      string   `json:"creation,omitempty"`
	Expiration    string   `json:"expiration,omitempty"`
	LastUse       string   `json:"lastUse,omitempty"`
	AllowedIPs    []string `json:"allowedIPs,omitempty"`
	Rules         []struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	} `json:"rules,omitempty"`
	Current bool `json:"current"`
}

func securityAccount(c *gin.Context, state *app.State) (string, bool) {
	accountID := strings.TrimSpace(c.Query("account"))
	if accountID == "" {
		accountID = strings.TrimSpace(c.Query("account_id"))
	}
	if _, ok := state.FindAccount(accountID); !ok || accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少有效账户"})
		return "", false
	}
	return accountID, true
}

// GetAPICredentials GET /api/ovh/security/credentials?account=
func GetAPICredentials(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		accountID, ok := securityAccount(c, state)
		if !ok {
			return
		}
		client, err := state.OVH.IsolatedClientFor(accountID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
		defer cancel()
		var current struct {
			CredentialID int64 `json:"credentialId"`
		}
		_ = client.GetWithContext(ctx, "/auth/currentCredential", &current)
		var ids []int64
		if err := client.GetWithContext(ctx, "/me/api/credential", &ids); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "无法读取 API 凭据列表: " + ovh.Explain(err)})
			return
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
		truncated := len(ids) > maxCredentialsListed
		if truncated {
			ids = ids[:maxCredentialsListed]
		}
		creds := make([]securityCredential, len(ids))
		apps := map[int64]struct{ name, desc string }{}
		var appsMu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 6)
		for i, id := range ids {
			wg.Add(1)
			go func(i int, id int64) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				isoClient, err := ovh.IsolateClient(client)
				if err != nil {
					creds[i] = securityCredential{CredentialID: id, Status: "unknown"}
					return
				}
				var cr securityCredential
				credPath, pErr := buildOVHPath("me", "api", "credential", strconv.FormatInt(id, 10))
				if pErr != nil || isoClient.GetWithContext(ctx, credPath, &cr) != nil {
					cr = securityCredential{CredentialID: id, Status: "unknown"}
				}
				cr.CredentialID = id
				cr.Current = id == current.CredentialID
				if cr.ApplicationID > 0 {
					appsMu.Lock()
					a, seen := apps[cr.ApplicationID]
					appsMu.Unlock()
					if !seen {
						var app struct {
							Name        string `json:"name"`
							Description string `json:"description"`
						}
						appPath, aErr := buildOVHPath("me", "api", "credential", strconv.FormatInt(id, 10), "application")
						if aErr == nil && isoClient.GetWithContext(ctx, appPath, &app) == nil {
							a = struct{ name, desc string }{app.Name, app.Description}
							appsMu.Lock()
							apps[cr.ApplicationID] = a
							appsMu.Unlock()
						}
					}
					cr.AppName, cr.AppDesc = a.name, a.desc
				}
				creds[i] = cr
			}(i, id)
		}
		wg.Wait()
		c.JSON(http.StatusOK, gin.H{"credentials": creds, "currentCredentialId": current.CredentialID, "truncated": truncated})
	}
}

// RevokeAPICredential DELETE /api/ovh/security/credentials/:credentialId?account_id=
// 吊销一条 API 凭据。本程序当前使用的凭据拒绝吊销。
func RevokeAPICredential(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		accountID, ok := securityAccount(c, state)
		if !ok {
			return
		}
		id, err := strconv.ParseInt(c.Param("credentialId"), 10, 64)
		if err != nil || id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "凭据编号不正确"})
			return
		}
		client, err := state.OVH.IsolatedClientFor(accountID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()
		var current struct {
			CredentialID int64 `json:"credentialId"`
		}
		if err := client.GetWithContext(ctx, "/auth/currentCredential", &current); err != nil {
			// 确认不了"是不是自己正在用的"就不吊销
			c.JSON(http.StatusBadGateway, gin.H{"error": "无法确认当前使用的凭据,已取消吊销: " + ovh.Explain(err)})
			return
		}
		if current.CredentialID == id {
			c.JSON(http.StatusConflict, gin.H{"error": "这是本程序正在使用的凭据,吊销后本程序将无法访问该账户,已拒绝"})
			return
		}
		credPath, ok := ovhPathOrAbort(c, "me", "api", "credential", strconv.FormatInt(id, 10))
		if !ok {
			return
		}
		if err := client.DeleteWithContext(ctx, credPath, nil); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "吊销失败: " + ovh.Explain(err)})
			return
		}
		state.Logger.Warn(fmt.Sprintf("已吊销账户 %s 的 API 凭据 %d", accountID, id), "security")
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// GetAPILogs GET /api/ovh/security/api-logs?account_id=
// 本账户最近的 API 调用记录(GET /me/api/logs/self),取最新的 30 条。
// 这个接口只有 EU 站点有(CA / US 的官方 schema 里没有),其他站点直接说明不支持。
func GetAPILogs(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		accountID, ok := securityAccount(c, state)
		if !ok {
			return
		}
		if acc, _ := state.FindAccount(accountID); ovh.EndpointRegion(acc.Endpoint) != "EU" {
			c.JSON(http.StatusOK, gin.H{"logs": []interface{}{}, "unsupported": true})
			return
		}
		client, err := state.OVH.IsolatedClientFor(accountID)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
		defer cancel()
		var ids []int64
		if err := client.GetWithContext(ctx, "/me/api/logs/self", &ids); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "无法读取 API 调用记录: " + ovh.Explain(err)})
			return
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
		if len(ids) > maxAPILogsListed {
			ids = ids[:maxAPILogsListed]
		}
		type apiLog struct {
			LogID  int64  `json:"logId"`
			Date   string `json:"date"`
			Method string `json:"method"`
			Path   string `json:"path"`
			Route  string `json:"route"`
			IP     string `json:"ip"`
		}
		logs := make([]apiLog, len(ids))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 6)
		for i, id := range ids {
			wg.Add(1)
			go func(i int, id int64) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				var l apiLog
				isoClient, err := ovh.IsolateClient(client)
				if err == nil {
					logPath, pErr := buildOVHPath("me", "api", "logs", "self", strconv.FormatInt(id, 10))
					if pErr != nil || isoClient.GetWithContext(ctx, logPath, &l) != nil {
						l = apiLog{}
					}
				}
				l.LogID = id
				logs[i] = l
			}(i, id)
		}
		wg.Wait()
		c.JSON(http.StatusOK, gin.H{"logs": logs})
	}
}
