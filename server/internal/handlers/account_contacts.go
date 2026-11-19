package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// contactChangeUnsupported US 区没有 /me/task/contactChange 系列端点:
// EU / CA schema 有全部 5 条(contactChange 本体 + accept/refuse/resendEmail),
// US 的 /me 只有 emailChange —— 本地 schema 与 live api.us.ovhcloud.com/1.0/me.json 都确认。
// 直接告诉调用方"这个区没有这个能力",不要把 OVH 的 404 包成 500 内部错误让人以为后端坏了。
//
// 判大区走 ovh.EndpointRegion 而不是比 "ovh-us" 字符串:endpoint 是用户可填字段,
// 大小写、以及将来可能出现的同大区别名都该按同一份归属表判断。
func contactChangeUnsupported(state *app.State, c *gin.Context) bool {
	acc, ok := ovhAccountFor(state, c)
	if !ok || ovh.EndpointRegion(acc.Endpoint) != "US" {
		return false
	}
	state.Logger.Warn("账户 "+acc.Name+" 属于 US 区,OVH US API 不提供联系人变更(contactChange)能力", "server_control")
	c.JSON(http.StatusNotImplemented, gin.H{
		"status":  "unsupported",
		"message": "OVH US 区不支持联系人变更请求,该功能仅在 EU / CA 区可用", "code": "E8015BE9B",
	})
	return true
}

// contactTokenTip 这个接口的 token 填错时补一句"下一步做什么"。
//
// schema 写得很明确:body 的 token 是
// "The token you received by email for this request" —— 邮件里的一次性确认令牌,
// 和 API 凭据没有任何关系。实测填错时 OVH 回的是 403 + "Invalid token",
// 光看状态码很容易误判成权限问题(Explain 曾经就是这么误判的)。
// 界面上正好有「重发邮件」,直接把用户指过去。
func contactTokenTip(err error) string {
	if err == nil {
		return ""
	}
	if strings.Contains(strings.ToLower(err.Error()), "token") {
		return "。这个 token 是 OVH 发到邮箱里的确认令牌(不是 API 密钥)," +
			"过期或用过都会失败 —— 可以点「重发邮件」拿一封新的再试"
	}
	return ""
}

// parseContactTaskID schema 里 id 是必填 long,0 / 负数 / 非数字都不是合法任务 id。
// 早点回 400,别拿 0 去打 OVH 换回一个 404 再包成 500(accept/refuse 还会顺带把用户的 token 发出去)。
func parseContactTaskID(c *gin.Context) (int64, bool) {
	taskID, err := strconv.ParseInt(c.Param("task_id"), 10, 64)
	if err != nil || taskID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "任务 ID 无效:必须是正整数", "code": "EB9AE459D"})
		return 0, false
	}
	return taskID, true
}

// GetContactChangeRequests GET /api/ovh/contact-change-requests
func GetContactChangeRequests(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if contactChangeUnsupported(state, c) {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var ids []interface{}
		if err := client.Get("/me/task/contactChange", &ids); err != nil {
			state.Logger.Error("获取联系人变更请求列表失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "获取联系人变更请求列表失败: " + ovh.Explain(err)})
			return
		}
		// 并发拉详情
		details, failed, firstErr := parallelGetDetailsCounted(client, ids, func(k interface{}) string {
			return safePathFn("me", "task", "contactChange", idToString(k))
		}, 10)
		list := collectDetails(state, c, details, failed, firstErr, "联系人变更请求", "server_control")
		if c.IsAborted() {
			return
		}
		sort.SliceStable(list, func(i, j int) bool {
			a, _ := list[i]["dateRequest"].(string)
			b, _ := list[j]["dateRequest"].(string)
			return a > b
		})
		state.Logger.Info(fmt.Sprintf("成功获取 %d 个联系人变更请求(共 %d 个,失败 %d 个)", len(list), len(ids), failed), "server_control")
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": list, "total": len(ids), "failed": failed})
	}
}

// GetContactChangeRequestDetail GET /api/ovh/contact-change-requests/:task_id
func GetContactChangeRequestDetail(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if contactChangeUnsupported(state, c) {
			return
		}
		taskID, ok := parseContactTaskID(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var d map[string]interface{}
		detailPath, ok := ovhPathOrAbort(c, "me", "task", "contactChange", strconv.FormatInt(taskID, 10))
		if !ok {
			return
		}
		if err := client.Get(detailPath, &d); err != nil {
			state.Logger.Error(fmt.Sprintf("获取联系人变更请求 %d 详情失败: %s", taskID, err.Error()), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "获取联系人变更请求详情失败: " + ovh.Explain(err)})
			return
		}
		state.Logger.Info(fmt.Sprintf("成功获取联系人变更请求 %d 详情", taskID), "server_control")
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": d})
	}
}

// AcceptContactChangeRequest POST /api/ovh/contact-change-requests/:task_id/accept
func AcceptContactChangeRequest(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if contactChangeUnsupported(state, c) {
			return
		}
		taskID, ok := parseContactTaskID(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var body struct {
			Token string `json:"token"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "缺少必需的 token 参数。请从邮件中获取 token 并输入。", "code": "EBC948D95"})
			return
		}
		acceptPath, ok := ovhPathOrAbort(c, "me", "task", "contactChange", strconv.FormatInt(taskID, 10), "accept")
		if !ok {
			return
		}
		if err := client.Post(acceptPath, map[string]interface{}{
			"token": body.Token,
		}, nil); err != nil {
			state.Logger.Error(fmt.Sprintf("接受联系人变更请求 %d 失败: %s", taskID, err.Error()), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "接受联系人变更请求失败: " + ovh.Explain(err) + contactTokenTip(err),
			})
			return
		}
		state.Logger.Info(fmt.Sprintf("成功接受联系人变更请求 %d", taskID), "server_control")
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "联系人变更请求已接受", "code": "E87E768E8"})
	}
}

// RefuseContactChangeRequest POST /api/ovh/contact-change-requests/:task_id/refuse
func RefuseContactChangeRequest(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if contactChangeUnsupported(state, c) {
			return
		}
		taskID, ok := parseContactTaskID(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var body struct {
			Token string `json:"token"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "缺少必需的 token 参数。请从邮件中获取 token 并输入。", "code": "EBC948D95"})
			return
		}
		refusePath, ok := ovhPathOrAbort(c, "me", "task", "contactChange", strconv.FormatInt(taskID, 10), "refuse")
		if !ok {
			return
		}
		if err := client.Post(refusePath, map[string]interface{}{
			"token": body.Token,
		}, nil); err != nil {
			state.Logger.Error(fmt.Sprintf("拒绝联系人变更请求 %d 失败: %s", taskID, err.Error()), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "拒绝联系人变更请求失败: " + ovh.Explain(err) + contactTokenTip(err),
			})
			return
		}
		state.Logger.Info(fmt.Sprintf("成功拒绝联系人变更请求 %d", taskID), "server_control")
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "联系人变更请求已拒绝", "code": "E30A15DE3"})
	}
}

// ResendContactChangeEmail POST /api/ovh/contact-change-requests/:task_id/resend-email
func ResendContactChangeEmail(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if contactChangeUnsupported(state, c) {
			return
		}
		taskID, ok := parseContactTaskID(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		resendPath, ok := ovhPathOrAbort(c, "me", "task", "contactChange", strconv.FormatInt(taskID, 10), "resendEmail")
		if !ok {
			return
		}
		if err := client.Post(resendPath, map[string]interface{}{}, nil); err != nil {
			state.Logger.Error(fmt.Sprintf("重发联系人变更请求 %d 邮件失败: %s", taskID, err.Error()), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "重发邮件失败: " + ovh.Explain(err)})
			return
		}
		state.Logger.Info(fmt.Sprintf("成功重发联系人变更请求 %d 的邮件", taskID), "server_control")
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "确认邮件已重新发送", "code": "E97D26804"})
	}
}
