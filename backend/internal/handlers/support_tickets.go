package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
)

// OVH 支持工单。官方路径是 /support/tickets，创建要打到 /support/tickets/create。
// 列表只返回 ID，详情和消息要再取。accountId 是客户号，不回给前端。

const (
	supportTicketPageDefault = 20
	supportTicketPageMax     = 50
	supportSubjectMax        = 255
	supportBodyMax           = 10000
)

var supportCategories = map[string]struct{}{
	"assistance": {},
	"billing":    {},
	"incident":   {},
}

var supportSubcategories = map[string]struct{}{
	"alerts": {}, "autorenew": {}, "bill": {}, "down": {}, "inProgress": {},
	"new": {}, "other": {}, "perfs": {}, "start": {}, "usage": {},
}

var supportProducts = map[string]struct{}{
	"adsl": {}, "cdn": {}, "dedicated": {}, "dedicated-billing": {}, "dedicated-other": {},
	"dedicatedcloud": {}, "domain": {}, "exchange": {}, "fax": {}, "hosting": {},
	"housing": {}, "iaas": {}, "mail": {}, "network": {}, "publiccloud": {},
	"sms": {}, "ssl": {}, "storage": {}, "telecom-billing": {}, "telecom-other": {},
	"vac": {}, "voip": {}, "vps": {}, "web-billing": {}, "web-other": {},
}

var supportStates = map[string]struct{}{
	"open": {}, "closed": {}, "unknown": {},
}

type supportTicket struct {
	TicketID        int64  `json:"ticketId"`
	TicketNumber    int64  `json:"ticketNumber"`
	Subject         string `json:"subject"`
	State           string `json:"state"`
	Category        string `json:"category,omitempty"`
	Product         string `json:"product,omitempty"`
	ServiceName     string `json:"serviceName,omitempty"`
	LastMessageFrom string `json:"lastMessageFrom"`
	CanBeClosed     bool   `json:"canBeClosed"`
	CreationDate    string `json:"creationDate"`
	UpdateDate      string `json:"updateDate"`
	Type            string `json:"type,omitempty"`
	Score           string `json:"score,omitempty"`
}

type supportMessage struct {
	MessageID    int64  `json:"messageId"`
	TicketID     int64  `json:"ticketId"`
	Body         string `json:"body"`
	From         string `json:"from"`
	CreationDate string `json:"creationDate"`
	UpdateDate   string `json:"updateDate"`
}

type supportNewTicket struct {
	AdditionalNotice string `json:"additionalNotice"`
	MessageID        int64  `json:"messageId"`
	TicketID         int64  `json:"ticketId"`
	TicketNumber     int64  `json:"ticketNumber"`
}

type supportCreateRequest struct {
	Subject     string `json:"subject"`
	Body        string `json:"body"`
	Category    string `json:"category"`
	Subcategory string `json:"subcategory"`
	Product     string `json:"product"`
	ServiceName string `json:"serviceName"`
}

type supportTextRequest struct {
	Body string `json:"body"`
}

// ListSupportTickets GET /api/support/tickets
func ListSupportTickets(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		query, err := supportTicketListQuery(c.Query("status"), c.Query("q"), c.Query("archived"), c.Query("page"), c.Query("pageSize"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		var ids []int64
		if err := client.Get("/support/tickets?"+query.Encode(), &ids); err != nil {
			respondOVHError(c, err)
			return
		}
		tickets, errs := fetchSupportTickets(client, ids)
		failed, firstErr := countErrs(errs)
		if len(ids) > 0 && len(tickets) == 0 {
			if firstErr == nil {
				firstErr = fmt.Errorf("没有读到工单详情")
			}
			respondOVHError(c, firstErr)
			return
		}
		sort.SliceStable(tickets, func(i, j int) bool {
			return ticketTime(tickets[i].UpdateDate).After(ticketTime(tickets[j].UpdateDate))
		})
		resp := gin.H{"success": true, "tickets": tickets}
		if failed > 0 {
			resp["incomplete"] = true
			resp["warning"] = fmt.Sprintf("有 %d 张工单没有读到详情", failed)
		}
		c.JSON(http.StatusOK, resp)
	}
}

// GetSupportTicket GET /api/support/tickets/:ticket_id
func GetSupportTicket(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseSupportTicketID(c.Param("ticket_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		var ticket supportTicket
		if err := client.Get(fmt.Sprintf("/support/tickets/%s", id), &ticket); err != nil {
			respondOVHError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "ticket": ticket})
	}
}

// ListSupportTicketMessages GET /api/support/tickets/:ticket_id/messages
func ListSupportTicketMessages(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseSupportTicketID(c.Param("ticket_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		var messages []supportMessage
		if err := client.Get(fmt.Sprintf("/support/tickets/%s/messages", id), &messages); err != nil {
			respondOVHError(c, err)
			return
		}
		if messages == nil {
			messages = []supportMessage{}
		}
		sort.SliceStable(messages, func(i, j int) bool {
			if messages[i].CreationDate == messages[j].CreationDate {
				return messages[i].MessageID < messages[j].MessageID
			}
			return ticketTime(messages[i].CreationDate).Before(ticketTime(messages[j].CreationDate))
		})
		c.JSON(http.StatusOK, gin.H{"success": true, "messages": messages})
	}
}

// CreateSupportTicket POST /api/support/tickets
func CreateSupportTicket(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req supportCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请求体不是合法的 JSON"})
			return
		}
		payload, err := buildSupportCreatePayload(req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		var created supportNewTicket
		if err := client.Post("/support/tickets/create", payload, &created); err != nil {
			respondOVHError(c, err)
			return
		}
		resp := gin.H{
			"success":      true,
			"ticketId":     created.TicketID,
			"ticketNumber": created.TicketNumber,
			"messageId":    created.MessageID,
		}
		if notice := strings.TrimSpace(created.AdditionalNotice); notice != "" {
			resp["additionalNotice"] = notice
		}
		c.JSON(http.StatusOK, resp)
	}
}

// ReplySupportTicket POST /api/support/tickets/:ticket_id/reply
func ReplySupportTicket(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, body, client, ok := supportTextAction(state, c)
		if !ok {
			return
		}
		if err := client.Post(fmt.Sprintf("/support/tickets/%s/reply", id), map[string]string{"body": body}, nil); err != nil {
			respondOVHError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// ReopenSupportTicket POST /api/support/tickets/:ticket_id/reopen
// 官方接口重新打开时必须带一条说明。
func ReopenSupportTicket(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, body, client, ok := supportTextAction(state, c)
		if !ok {
			return
		}
		if err := client.Post(fmt.Sprintf("/support/tickets/%s/reopen", id), map[string]string{"body": body}, nil); err != nil {
			respondOVHError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// CloseSupportTicket POST /api/support/tickets/:ticket_id/close
func CloseSupportTicket(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := parseSupportTicketID(c.Param("ticket_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		if err := client.Post(fmt.Sprintf("/support/tickets/%s/close", id), nil, nil); err != nil {
			respondOVHError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

func supportTextAction(state *app.State, c *gin.Context) (string, string, *ovhsdk.Client, bool) {
	id, err := parseSupportTicketID(c.Param("ticket_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return "", "", nil, false
	}
	var req supportTextRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请求体不是合法的 JSON"})
		return "", "", nil, false
	}
	body, err := cleanSupportText(req.Body, supportBodyMax, "回复内容")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return "", "", nil, false
	}
	client, err := ovhClientFor(state, c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return "", "", nil, false
	}
	return id, body, client, true
}

func supportTicketListQuery(status, q, archived, pageRaw, pageSizeRaw string) (url.Values, error) {
	values := url.Values{}
	status = strings.TrimSpace(status)
	if status != "" && status != "all" {
		if _, ok := supportStates[status]; !ok {
			return nil, fmt.Errorf("状态只能是 open、closed 或 unknown")
		}
		values.Set("status", status)
	}
	switch strings.TrimSpace(strings.ToLower(archived)) {
	case "", "false", "0":
	case "true", "1":
		values.Set("archived", "true")
	default:
		return nil, fmt.Errorf("archived 只能是 true 或 false")
	}
	page, err := supportPage(pageRaw, 1)
	if err != nil {
		return nil, err
	}
	pageSize, err := supportPage(pageSizeRaw, supportTicketPageDefault)
	if err != nil {
		return nil, err
	}
	if pageSize > supportTicketPageMax {
		pageSize = supportTicketPageMax
	}
	if pageSize < 1 {
		return nil, fmt.Errorf("pageSize 至少为 1")
	}
	values.Set("page", strconv.Itoa(page))
	values.Set("pageSize", strconv.Itoa(pageSize))
	q = strings.TrimSpace(q)
	if q != "" {
		if supportAllDigits(q) {
			values.Set("ticketNumber", q)
		} else {
			values.Set("subject", q)
		}
	}
	return values, nil
}

func supportPage(raw string, fallback int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("页码和每页数量必须是正整数")
	}
	return n, nil
}

func buildSupportCreatePayload(req supportCreateRequest) (map[string]string, error) {
	subject, err := cleanSupportText(req.Subject, supportSubjectMax, "标题")
	if err != nil {
		return nil, err
	}
	body, err := cleanSupportText(req.Body, supportBodyMax, "内容")
	if err != nil {
		return nil, err
	}
	payload := map[string]string{
		"subject": subject,
		"body":    body,
	}
	if err := putSupportEnum(payload, "category", req.Category, supportCategories, "分类"); err != nil {
		return nil, err
	}
	if err := putSupportEnum(payload, "subcategory", req.Subcategory, supportSubcategories, "子分类"); err != nil {
		return nil, err
	}
	if err := putSupportEnum(payload, "product", req.Product, supportProducts, "产品"); err != nil {
		return nil, err
	}
	service := strings.TrimSpace(req.ServiceName)
	if service != "" {
		if err := validateSupportServiceName(service); err != nil {
			return nil, err
		}
		payload["serviceName"] = service
	}
	return payload, nil
}

func putSupportEnum(dst map[string]string, key, value string, allowed map[string]struct{}, label string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if _, ok := allowed[value]; !ok {
		return fmt.Errorf("%s不是 OVH 接受的值", label)
	}
	dst[key] = value
	return nil
}

func cleanSupportText(raw string, max int, label string) (string, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", fmt.Errorf("%s不能为空", label)
	}
	if utf8.RuneCountInString(text) > max {
		return "", fmt.Errorf("%s不能超过 %d 个字符", label, max)
	}
	return text, nil
}

func validateSupportServiceName(svc string) error {
	if len(svc) > maxServiceNameLen {
		return fmt.Errorf("服务名过长")
	}
	for _, r := range svc {
		ok := r == '.' || r == '-' || r == '_' ||
			(r >= '0' && r <= '9') ||
			(r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z')
		if !ok {
			return fmt.Errorf("服务名只能包含字母、数字、点、连字符和下划线")
		}
	}
	return nil
}

func parseSupportTicketID(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 18 || raw[0] == '0' || !supportAllDigits(raw) {
		return "", fmt.Errorf("工单号不正确")
	}
	return raw, nil
}

func supportAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func fetchSupportTickets(client *ovhsdk.Client, ids []int64) ([]supportTicket, []error) {
	tickets := make([]supportTicket, len(ids))
	ok := make([]bool, len(ids))
	errs := make([]error, len(ids))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, ticketID int64) {
			defer wg.Done()
			defer func() { <-sem }()
			var ticket supportTicket
			if err := client.Get(fmt.Sprintf("/support/tickets/%d", ticketID), &ticket); err != nil {
				errs[idx] = err
				return
			}
			if ticket.TicketID == 0 {
				ticket.TicketID = ticketID
			}
			tickets[idx] = ticket
			ok[idx] = true
		}(i, id)
	}
	wg.Wait()
	out := make([]supportTicket, 0, len(ids))
	for i := range tickets {
		if ok[i] {
			out = append(out, tickets[i])
		}
	}
	return out, errs
}



func ticketTime(raw string) time.Time {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return t
}
