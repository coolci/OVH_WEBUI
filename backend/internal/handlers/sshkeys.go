package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/app"
)

// ListSSHKeys GET /api/sshkeys
// 获取当前账户所有全局预存 SSH 密钥
func ListSSHKeys(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		var keyNames []string
		if err := client.Get("/me/sshKey", &keyNames); err != nil {
			state.Logger.Error("获取 SSH 密钥列表失败: "+err.Error(), "ssh_keys")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "获取 SSH 密钥列表失败: " + err.Error()})
			return
		}

		if len(keyNames) == 0 {
			c.JSON(http.StatusOK, gin.H{"success": true, "keys": []map[string]interface{}{}})
			return
		}

		details, _ := parallelGetStringKeysWithErrs(client, keyNames, func(name string) string {
			return "/me/sshKey/" + url.PathEscape(name)
		}, 10)

		var list []map[string]interface{}
		for i, d := range details {
			if d != nil {
				list = append(list, d)
			} else {
				list = append(list, map[string]interface{}{
					"keyName": keyNames[i],
				})
			}
		}

		sort.Slice(list, func(i, j int) bool {
			defI, _ := list[i]["default"].(bool)
			defJ, _ := list[j]["default"].(bool)
			if defI != defJ {
				return defI
			}
			nI, _ := list[i]["keyName"].(string)
			nJ, _ := list[j]["keyName"].(string)
			return nI < nJ
		})

		c.JSON(http.StatusOK, gin.H{"success": true, "keys": list})
	}
}

// CreateSSHKey POST /api/sshkeys
// 添加一条全局 SSH 公钥到 OVH 账户
func CreateSSHKey(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			KeyName string `json:"keyName"`
			Key     string `json:"key"`
			Default bool   `json:"default"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请求参数格式错误: " + err.Error()})
			return
		}

		name := strings.TrimSpace(body.KeyName)
		key := strings.TrimSpace(body.Key)
		if name == "" || key == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "keyName 与 key 不能为空"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		payload := map[string]interface{}{
			"keyName": name,
			"key":     key,
			"default": body.Default,
		}

		var result map[string]interface{}
		if err := client.Post("/me/sshKey", payload, &result); err != nil {
			state.Logger.Error(fmt.Sprintf("添加 SSH 密钥 %s 失败: %s", name, err.Error()), "ssh_keys")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "添加 SSH 密钥失败: " + err.Error()})
			return
		}

		state.Logger.Info(fmt.Sprintf("添加 SSH 密钥 %s 成功", name), "ssh_keys")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "SSH 密钥添加成功", "result": result})
	}
}

// DeleteSSHKey DELETE /api/sshkeys/:keyName
// 从账户删除指定 SSH 密钥
func DeleteSSHKey(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		keyName := strings.TrimSpace(c.Param("keyName"))
		if keyName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 keyName 参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		if err := client.Delete("/me/sshKey/"+url.PathEscape(keyName), nil); err != nil {
			state.Logger.Error(fmt.Sprintf("删除 SSH 密钥 %s 失败: %s", keyName, err.Error()), "ssh_keys")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "删除 SSH 密钥失败: " + err.Error()})
			return
		}

		state.Logger.Info(fmt.Sprintf("删除 SSH 密钥 %s 成功", keyName), "ssh_keys")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "SSH 密钥已删除"})
	}
}
