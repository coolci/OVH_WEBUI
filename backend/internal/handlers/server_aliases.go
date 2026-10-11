package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/platform/httperr"
)

// ListServerAliases GET /api/server-control/aliases[?account=<id>]
func ListServerAliases(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		acc, ok := ovhAccountFor(state, c)
		if !ok {
			httperr.PreconditionFailed(c, "未配置 OVH 账户", "NO_OVH_ACCOUNT")
			return
		}
		svc := state.ServerService
		m, err := svc.ListAliases(acc.ID)
		if err != nil {
			state.Logger.Error("list aliases: "+err.Error(), "server_control")
			httperr.InternalError(c, err.Error(), "INTERNAL_ERROR")
			return
		}
		if m == nil {
			m = map[string]string{}
		}
		c.JSON(http.StatusOK, m)
	}
}

// SetServerAlias PUT /api/server-control/:service_name/alias[?account=<id>]
func SetServerAlias(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		acc, ok := ovhAccountFor(state, c)
		if !ok {
			httperr.PreconditionFailed(c, "未配置 OVH 账户", "NO_OVH_ACCOUNT")
			return
		}
		svcName := strings.TrimSpace(c.Param("service_name"))
		if svcName == "" {
			httperr.BadRequest(c, "缺少 service_name", "E6803C9EF")
			return
		}
		var body struct {
			Alias string `json:"alias"`
		}
		_ = c.ShouldBindJSON(&body)
		alias := strings.TrimSpace(body.Alias)

		svc := state.ServerService
		if err := svc.SetAlias(acc.ID, svcName, alias); err != nil {
			state.Logger.Error("set alias: "+err.Error(), "server_control")
			httperr.InternalError(c, err.Error(), "INTERNAL_ERROR")
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "service_name": svcName, "alias": alias})
	}
}

// DeleteServerAlias DELETE /api/server-control/:service_name/alias[?account=<id>]
func DeleteServerAlias(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		acc, ok := ovhAccountFor(state, c)
		if !ok {
			httperr.PreconditionFailed(c, "未配置 OVH 账户", "NO_OVH_ACCOUNT")
			return
		}
		svcName := strings.TrimSpace(c.Param("service_name"))
		if svcName == "" {
			httperr.BadRequest(c, "缺少 service_name", "E6803C9EF")
			return
		}
		svc := state.ServerService
		if err := svc.DeleteAlias(acc.ID, svcName); err != nil {
			state.Logger.Error("delete alias: "+err.Error(), "server_control")
			httperr.InternalError(c, err.Error(), "INTERNAL_ERROR")
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}
