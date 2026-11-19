package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// Backup FTP GET
func GetBackupFTP(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		if featureBackupFTPUnavailable(state, c) {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "backupFTP")
		if !ok {
			return
		}
		var d map[string]interface{}
		if err := client.Get(p, &d); err != nil {
			code, msg := featureOVHErr(err)
			if code == http.StatusNotFound || strings.Contains(strings.ToLower(msg), "does not exist") {
				if featureIsFeatureMissing(client, svc) {
					c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "备份FTP未激活", "code": "E9C2BD021", "notActivated": true})
					return
				}
				state.Logger.Warn("查询服务器 "+svc+" 备份FTP：服务器不存在或不属于当前账户 - "+msg, "server_control")
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"error":   "服务器不存在或不属于当前账户", "code": "E834268F5",
					"unknownService": true,
					"reason":         msg,
				})
				return
			}
			state.Logger.Error("查询服务器 "+svc+" 备份FTP失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "backupFtp": d})
	}
}

// Backup FTP POST (activate)
func ActivateBackupFTP(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		if featureBackupFTPUnavailable(state, c) {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "backupFTP")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Post(p, map[string]interface{}{}, &result); err != nil {
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "cannot benefit") || strings.Contains(lower, "not available") {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "该服务器无法使用备份FTP服务", "code": "E826B0844",
					"notAvailable": true,
					"reason":       ovh.Explain(err),
				})
				return
			}
			state.Logger.Error("激活服务器 "+svc+" 备份FTP失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		state.Logger.Info("激活服务器 "+svc+" 备份FTP成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "备份FTP已激活", "code": "EE5FB27CE", "result": result})
	}
}

// Backup FTP DELETE
func DeleteBackupFTP(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		if featureBackupFTPUnavailable(state, c) {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "backupFTP")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Delete(p, &result); err != nil {
			state.Logger.Error("删除服务器 "+svc+" 备份FTP失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		state.Logger.Info("删除服务器 "+svc+" 备份FTP成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "备份FTP已删除", "code": "EB5E5B03F", "result": result})
	}
}

// Backup FTP access list
func GetBackupFTPAccess(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		if featureBackupFTPUnavailable(state, c) {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "backupFTP", "access")
		if !ok {
			return
		}
		var blocks []string
		if err := client.Get(p, &blocks); err != nil {
			code, msg := featureOVHErr(err)
			if code == http.StatusNotFound || strings.Contains(strings.ToLower(msg), "backupftp does not exist") || (code == http.StatusNotFound && strings.Contains(strings.ToLower(msg), "does not exist")) {
				if featureIsFeatureMissing(client, svc) {
					c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "备份FTP未激活", "code": "E9C2BD021", "notActivated": true})
					return
				}
			}
			state.Logger.Error("获取服务器 "+svc+" 备份FTP授权IP列表失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		// 并发拉每个 IP block 的详情
		details, errs := featureParallelACLDetails(client, svc, blocks, 10)
		list := []interface{}{}
		failed := 0
		var firstErr error
		for i, b := range blocks {
			if errs[i] != nil {
				failed++
				if firstErr == nil {
					firstErr = errs[i]
				}
				_, msg := featureOVHErr(errs[i])
				state.Logger.Warn("获取备份FTP授权IP "+b+" 详情失败: "+errs[i].Error(), "server_control")
				list = append(list, map[string]interface{}{"ipBlock": b, "error": msg})
				continue
			}
			if details[i] == nil {
				list = append(list, map[string]interface{}{"ipBlock": b})
				continue
			}
			list = append(list, details[i])
		}
		// 全挂通常是凭据失效/限流/OVH 故障，这种时候不能伪装成 success:true 让前端把占位对象当成真实数据
		if failed > 0 && failed == len(blocks) {
			state.Logger.Error("服务器 "+svc+" 备份FTP授权IP详情全部获取失败: "+firstErr.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(firstErr)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "accessList": list, "failedCount": failed})
	}
}

// Backup FTP access add
func AddBackupFTPAccess(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		if featureBackupFTPUnavailable(state, c) {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			IPBlock string `json:"ipBlock"`
			FTP     *bool  `json:"ftp"`
			NFS     bool   `json:"nfs"`
			CIFS    bool   `json:"cifs"`
		}
		_ = c.ShouldBindJSON(&body)
		body.IPBlock = strings.TrimSpace(body.IPBlock)
		if body.IPBlock == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少ipBlock参数", "code": "E1230A21F"})
			return
		}
		if _, ok := validateIPOrCIDR(body.IPBlock); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ipBlock 必须是合法的 IP 或 CIDR 网段", "code": "INVALID_IP_BLOCK"})
			return
		}
		ftp := true
		if body.FTP != nil {
			ftp = *body.FTP
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "backupFTP", "access")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Post(p, map[string]interface{}{
			"cifs":    body.CIFS,
			"ftp":     ftp,
			"ipBlock": body.IPBlock,
			"nfs":     body.NFS,
		}, &result); err != nil {
			state.Logger.Error("添加备份FTP访问IP "+body.IPBlock+" 失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		state.Logger.Info("添加备份FTP访问IP "+body.IPBlock+" 成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "访问IP已添加", "code": "E9495F9D0", "result": result})
	}
}

// Backup FTP access delete
func DeleteBackupFTPAccess(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		if featureBackupFTPUnavailable(state, c) {
			return
		}
		ipBlock := strings.TrimSpace(c.Query("ipBlock"))
		if ipBlock == "" {
			var body struct {
				IPBlock string `json:"ipBlock"`
			}
			_ = c.ShouldBindJSON(&body)
			ipBlock = strings.TrimSpace(body.IPBlock)
		}
		if ipBlock == "" {
			ipBlock = strings.TrimPrefix(strings.TrimSpace(c.Param("ip_block")), "/")
		}
		if ipBlock == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少ipBlock参数", "code": "E1230A21F"})
			return
		}
		if _, ok := validateIPOrCIDR(ipBlock); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ipBlock 参数格式不合法", "code": "INVALID_IP_BLOCK"})
			return
		}
		aclPath, pErr := featureBackupFTPACLPath(svc, ipBlock)
		if pErr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ipBlock 参数格式不合法", "code": "INVALID_IP_BLOCK"})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		if err := client.Delete(aclPath, nil); err != nil {
			state.Logger.Error("删除备份FTP访问IP "+ipBlock+" 失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		state.Logger.Info("删除备份FTP访问IP "+ipBlock+" 成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "访问IP已删除", "code": "E1E22216A"})
	}
}

// Backup FTP password
func ChangeBackupFTPPassword(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		if featureBackupFTPUnavailable(state, c) {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "backupFTP", "password")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Post(p, map[string]interface{}{}, &result); err != nil {
			state.Logger.Error("修改服务器 "+svc+" 备份FTP密码失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		state.Logger.Info("修改服务器 "+svc+" 备份FTP密码成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "密码已重置，新密码已发送至邮箱", "code": "E37D29005", "result": result})
	}
}

// Backup FTP authorizable blocks
func GetBackupFTPAuthorizableBlocks(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		if featureBackupFTPUnavailable(state, c) {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "backupFTP", "authorizableBlocks")
		if !ok {
			return
		}
		var blocks []string
		if err := client.Get(p, &blocks); err != nil {
			code, msg := featureOVHErr(err)
			if code == http.StatusNotFound || strings.Contains(strings.ToLower(msg), "backupftp does not exist") || (code == http.StatusNotFound && strings.Contains(strings.ToLower(msg), "does not exist")) {
				if featureIsFeatureMissing(client, svc) {
					c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "备份FTP未激活", "code": "E9C2BD021", "notActivated": true})
					return
				}
			}
			state.Logger.Error("获取服务器 "+svc+" 备份FTP可授权IP段失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "blocks": blocks})
	}
}
