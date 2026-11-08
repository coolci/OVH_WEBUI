package handlers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

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
	accountID := strings.TrimSpace(c.Query("account_id"))
	if _, ok := state.FindAccount(accountID); !ok || accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少有效账户"})
		return "", false
	}
	return accountID, true
}

// GetAPICredentials GET /api/ovh/security/credentials?account_id=
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
				var cr securityCredential
				if err := client.GetWithContext(ctx, fmt.Sprintf("/me/api/credential/%d", id), &cr); err != nil {
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
						if err := client.GetWithContext(ctx, fmt.Sprintf("/me/api/credential/%d/application", id), &app); err == nil {
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
		if err := client.DeleteWithContext(ctx, fmt.Sprintf("/me/api/credential/%d", id), nil); err != nil {
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
				if err := client.GetWithContext(ctx, fmt.Sprintf("/me/api/logs/self/%d", id), &l); err != nil {
					l = apiLog{}
				}
				l.LogID = id
				logs[i] = l
			}(i, id)
		}
		wg.Wait()
		c.JSON(http.StatusOK, gin.H{"logs": logs})
	}
}
