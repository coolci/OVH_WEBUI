package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
)

// ListTickets GET /api/tickets
// 获取账户名下的官方支持工单列表
func ListTickets(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		q := url.Values{}
		if archived := c.Query("archived"); archived != "" {
			q.Set("archived", archived)
		}

		apiPath := "/support/tickets"
		if len(q) > 0 {
			apiPath += "?" + q.Encode()
		}

		var ticketIDs []interface{}
		if err := client.Get(apiPath, &ticketIDs); err != nil {
			state.Logger.Error("获取工单列表失败: "+err.Error(), "support_tickets")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "获取工单列表失败: " + err.Error()})
			return
		}

		if len(ticketIDs) == 0 {
			c.JSON(http.StatusOK, gin.H{"success": true, "tickets": []map[string]interface{}{}})
			return
		}

		// 按 ticketId 降序排
		sort.Slice(ticketIDs, func(i, j int) bool {
			idI := idToInt64(ticketIDs[i])
			idJ := idToInt64(ticketIDs[j])
			return idI > idJ
		})

		limit := 40
		if raw := c.Query("limit"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}
		if len(ticketIDs) < limit {
			limit = len(ticketIDs)
		}

		details := parallelGetDetails(client, ticketIDs[:limit], func(k interface{}) string {
			return fmt.Sprintf("/support/tickets/%v", k)
		}, 10)

		var list []map[string]interface{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "tickets": list})
	}
}

// GetTicketDetail GET /api/tickets/:id
// 获取单个工单基本信息
func GetTicketDetail(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := strings.TrimSpace(c.Param("id"))
		if ticketID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 ticketId 参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		var detail map[string]interface{}
		if err := client.Get(fmt.Sprintf("/support/tickets/%s", ticketID), &detail); err != nil {
			state.Logger.Error(fmt.Sprintf("获取工单 %s 详情失败: %s", ticketID, err.Error()), "support_tickets")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "ticket": detail})
	}
}

// GetTicketMessages GET /api/tickets/:id/messages
// 获取工单的历史交互消息流
func GetTicketMessages(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := strings.TrimSpace(c.Param("id"))
		if ticketID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 ticketId 参数"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		var messages []map[string]interface{}
		if err := client.Get(fmt.Sprintf("/support/tickets/%s/messages", ticketID), &messages); err != nil {
			state.Logger.Error(fmt.Sprintf("获取工单 %s 消息流失败: %s", ticketID, err.Error()), "support_tickets")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}

		// 按时间升序排序
		sort.Slice(messages, func(i, j int) bool {
			timeI, _ := messages[i]["creationDate"].(string)
			timeJ, _ := messages[j]["creationDate"].(string)
			return timeI < timeJ
		})

		c.JSON(http.StatusOK, gin.H{"success": true, "messages": messages})
	}
}

// ReplyTicket POST /api/tickets/:id/reply
// 向工单追加回复内容
func ReplyTicket(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ticketID := strings.TrimSpace(c.Param("id"))
		if ticketID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 ticketId 参数"})
			return
		}

		var body struct {
			Body string `json:"body"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.Body) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "回复内容不能为空"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		payload := map[string]interface{}{
			"body": strings.TrimSpace(body.Body),
		}

		var result map[string]interface{}
		if err := client.Post(fmt.Sprintf("/support/tickets/%s/reply", ticketID), payload, &result); err != nil {
			state.Logger.Error(fmt.Sprintf("回复工单 %s 失败: %s", ticketID, err.Error()), "support_tickets")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "回复工单失败: " + err.Error()})
			return
		}

		state.Logger.Info(fmt.Sprintf("成功回复工单 %s", ticketID), "support_tickets")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "回复已发送", "result": result})
	}
}

// CreateTicket POST /api/tickets
// 创建并提交新支持工单
func CreateTicket(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Body        string `json:"body"`
			Category    string `json:"category"` // technical, billing, assistance
			Subject     string `json:"subject"`
			ServiceName string `json:"serviceName,omitempty"`
			Type        string `json:"type,omitempty"` // critical / generic
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请求参数格式错误: " + err.Error()})
			return
		}

		msgBody := strings.TrimSpace(body.Body)
		subj := strings.TrimSpace(body.Subject)
		cat := strings.TrimSpace(body.Category)
		if msgBody == "" || subj == "" || cat == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "主题、分类和内容均不能为空"})
			return
		}

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		payload := map[string]interface{}{
			"body":     msgBody,
			"category": cat,
			"subject":  subj,
		}
		if strings.TrimSpace(body.ServiceName) != "" {
			payload["serviceName"] = strings.TrimSpace(body.ServiceName)
		}
		if strings.TrimSpace(body.Type) != "" {
			payload["type"] = strings.TrimSpace(body.Type)
		}

		var result map[string]interface{}
		if err := client.Post("/support/tickets/create", payload, &result); err != nil {
			state.Logger.Error(fmt.Sprintf("创建工单失败: %s", err.Error()), "support_tickets")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "创建工单失败: " + err.Error()})
			return
		}

		ticketID := numconv.ToString(result["ticketId"])
		state.Logger.Info(fmt.Sprintf("成功创建工单 %s: %s", ticketID, subj), "support_tickets")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "工单创建成功",
			"result":  result,
		})
	}
}
