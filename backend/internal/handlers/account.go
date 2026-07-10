package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
)

func noOVHRespAccount(c *gin.Context) {
	msg := "未配置 OVH 账户或指定的账户不存在"
	if acc := c.Query("account"); acc != "" {
		msg = "OVH 账户不存在或已删除（account=" + acc + "），请在设置中重新选择账户"
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": msg,
		"code":  "OVH_ACCOUNT_MISSING",
	})
}

// GetAccountInfo GET /api/ovh/account/info
func GetAccountInfo(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var info map[string]interface{}
		if err := client.Get("/me", &info); err != nil {
			state.Logger.Error("获取账户信息失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取账户信息失败: " + err.Error()})
			return
		}
		state.Logger.Info("成功获取账户信息", "account_management")
		c.JSON(http.StatusOK, info)
	}
}

// GetAccountOrders GET /api/ovh/account/orders
// 拉取 OVH /me/order 最近订单详情（默认最多 30 条，?limit= 可调）
func GetAccountOrders(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var ids []interface{}
		if err := client.Get("/me/order", &ids); err != nil {
			state.Logger.Error("获取订单列表失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取订单列表失败: " + err.Error()})
			return
		}
		// 新订单通常 ID 更大，倒序取最近
		sort.Slice(ids, func(i, j int) bool {
			return idToInt64(ids[i]) > idToInt64(ids[j])
		})
		limit := 30
		if raw := c.Query("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 {
				limit = n
				if limit > 100 {
					limit = 100
				}
			}
		}
		if len(ids) < limit {
			limit = len(ids)
		}
		details := parallelGetDetails(client, ids[:limit], func(k interface{}) string {
			return "/me/order/" + idToString(k)
		}, 10)
		list := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}
		state.Logger.Info(fmt.Sprintf("成功获取 %d 条订单记录", len(list)), "account_management")
		c.JSON(http.StatusOK, list)
	}
}

// GetAccountRefunds GET /api/ovh/account/refunds
func GetAccountRefunds(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var ids []interface{}
		if err := client.Get("/me/refund", &ids); err != nil {
			state.Logger.Error("获取退款列表失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取退款列表失败: " + err.Error()})
			return
		}
		max := 20
		if len(ids) < max {
			max = len(ids)
		}
		// 并发拉详情：10 并发，原 20 * 200ms = 4 秒 -> 2 * 200ms = 0.4 秒
		details := parallelGetDetails(client, ids[:max], func(k interface{}) string {
			return "/me/refund/" + idToString(k)
		}, 10)
		list := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}
		state.Logger.Info(fmt.Sprintf("成功获取 %d 条退款记录", len(list)), "account_management")
		c.JSON(http.StatusOK, list)
	}
}

// GetCreditBalance GET /api/ovh/account/credit-balance
func GetCreditBalance(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var names []string
		if err := client.Get("/me/credit/balance", &names); err != nil {
			state.Logger.Error("获取信用余额失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "获取信用余额失败: " + err.Error()})
			return
		}
		// 并发拉详情
		details := parallelGetStringKeys(client, names, func(n string) string {
			return "/me/credit/balance/" + n
		}, 10)
		balances := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				balances = append(balances, d)
			}
		}
		state.Logger.Info(fmt.Sprintf("成功获取 %d 个信用余额", len(balances)), "account_management")
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": balances})
	}
}

// GetEmailHistory GET /api/ovh/account/email-history
func GetEmailHistory(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var ids []interface{}
		if err := client.Get("/me/notification/email/history", &ids); err != nil {
			state.Logger.Error("获取邮件历史失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取邮件历史失败: " + err.Error()})
			return
		}
		// 倒序
		for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
			ids[i], ids[j] = ids[j], ids[i]
		}
		max := 50
		if len(ids) < max {
			max = len(ids)
		}
		// 并发拉 50 封邮件详情：原 50 * 200ms = 10 秒 -> 5 * 200ms = 1 秒
		details := parallelGetDetails(client, ids[:max], func(k interface{}) string {
			return "/me/notification/email/history/" + idToString(k)
		}, 10)
		list := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}
		state.Logger.Info(fmt.Sprintf("成功获取 %d 封邮件（总共 %d 封）", len(list), len(ids)), "account_management")
		c.JSON(http.StatusOK, list)
	}
}

// GetContactChangeRequests GET /api/ovh/contact-change-requests
func GetContactChangeRequests(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var ids []interface{}
		if err := client.Get("/me/task/contactChange", &ids); err != nil {
			state.Logger.Error("获取联系人变更请求列表失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "获取联系人变更请求列表失败: " + err.Error()})
			return
		}
		// 并发拉详情
		details := parallelGetDetails(client, ids, func(k interface{}) string {
			return "/me/task/contactChange/" + idToString(k)
		}, 10)
		list := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}
		sort.SliceStable(list, func(i, j int) bool {
			a, _ := list[i]["dateRequest"].(string)
			b, _ := list[j]["dateRequest"].(string)
			return a > b
		})
		state.Logger.Info(fmt.Sprintf("成功获取 %d 个联系人变更请求", len(list)), "server_control")
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
	}
}

// GetContactChangeRequestDetail GET /api/ovh/contact-change-requests/:task_id
func GetContactChangeRequestDetail(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskIDStr := c.Param("task_id")
		taskID, _ := strconv.ParseInt(taskIDStr, 10, 64)
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var d map[string]interface{}
		if err := client.Get(fmt.Sprintf("/me/task/contactChange/%d", taskID), &d); err != nil {
			state.Logger.Error(fmt.Sprintf("获取联系人变更请求 %d 详情失败: %s", taskID, err.Error()), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "获取联系人变更请求详情失败: " + err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("成功获取联系人变更请求 %d 详情", taskID), "server_control")
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": d})
	}
}

// AcceptContactChangeRequest POST /api/ovh/contact-change-requests/:task_id/accept
func AcceptContactChangeRequest(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskIDStr := c.Param("task_id")
		taskID, _ := strconv.ParseInt(taskIDStr, 10, 64)
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
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "缺少必需的 token 参数。请从邮件中获取 token 并输入。"})
			return
		}
		if err := client.Post(fmt.Sprintf("/me/task/contactChange/%d/accept", taskID), map[string]interface{}{
			"token": body.Token,
		}, nil); err != nil {
			state.Logger.Error(fmt.Sprintf("接受联系人变更请求 %d 失败: %s", taskID, err.Error()), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "接受联系人变更请求失败: " + err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("成功接受联系人变更请求 %d", taskID), "server_control")
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "联系人变更请求已接受"})
	}
}

// RefuseContactChangeRequest POST /api/ovh/contact-change-requests/:task_id/refuse
func RefuseContactChangeRequest(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskIDStr := c.Param("task_id")
		taskID, _ := strconv.ParseInt(taskIDStr, 10, 64)
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
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "缺少必需的 token 参数。请从邮件中获取 token 并输入。"})
			return
		}
		if err := client.Post(fmt.Sprintf("/me/task/contactChange/%d/refuse", taskID), map[string]interface{}{
			"token": body.Token,
		}, nil); err != nil {
			state.Logger.Error(fmt.Sprintf("拒绝联系人变更请求 %d 失败: %s", taskID, err.Error()), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "拒绝联系人变更请求失败: " + err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("成功拒绝联系人变更请求 %d", taskID), "server_control")
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "联系人变更请求已拒绝"})
	}
}

// ResendContactChangeEmail POST /api/ovh/contact-change-requests/:task_id/resend-email
func ResendContactChangeEmail(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskIDStr := c.Param("task_id")
		taskID, _ := strconv.ParseInt(taskIDStr, 10, 64)
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		if err := client.Post(fmt.Sprintf("/me/task/contactChange/%d/resendEmail", taskID), map[string]interface{}{}, nil); err != nil {
			state.Logger.Error(fmt.Sprintf("重发联系人变更请求 %d 邮件失败: %s", taskID, err.Error()), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "重发邮件失败: " + err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("成功重发联系人变更请求 %d 的邮件", taskID), "server_control")
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "确认邮件已重新发送"})
	}
}

// GetSubAccounts GET /api/ovh/account/sub-accounts
func GetSubAccounts(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var ids []interface{}
		if err := client.Get("/me/subAccount", &ids); err != nil {
			state.Logger.Error("获取子账户列表失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "获取子账户列表失败: " + err.Error()})
			return
		}
		// 并发拉详情
		details := parallelGetDetails(client, ids, func(k interface{}) string {
			return "/me/subAccount/" + idToString(k)
		}, 10)
		list := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}
		state.Logger.Info(fmt.Sprintf("成功获取 %d 个子账户", len(list)), "account_management")
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
	}
}

// GetAccountBills GET /api/ovh/account/bills
func GetAccountBills(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHRespAccount(c)
			return
		}
		var ids []string
		if err := client.Get("/me/bill", &ids); err != nil {
			state.Logger.Error("获取账单列表失败: "+err.Error(), "account_management")
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "获取账单列表失败: " + err.Error()})
			return
		}
		max := 20
		if len(ids) < max {
			max = len(ids)
		}
		// 并发拉 20 个账单详情
		details := parallelGetStringKeys(client, ids[:max], func(s string) string {
			return "/me/bill/" + s
		}, 10)
		list := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}
		state.Logger.Info(fmt.Sprintf("成功获取 %d 条账单记录", len(list)), "account_management")
		c.JSON(http.StatusOK, gin.H{"status": "success", "data": list})
	}
}
