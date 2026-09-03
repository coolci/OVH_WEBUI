package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/ovh-webui/server/internal/app"
)

// VerifyConfig 检查 Telegram 是否可用:Token / Chat ID 是否填写 + bot 是否能 getMe + chat 是否可访问。
// 用于 AddSubscription 等"必须 TG 有效"的强制校验。
// 返回 (ok, 失败原因)。所有失败原因都是面向终端用户的中文短句。
func VerifyConfig(state *app.State) (bool, string) {
	cfg := state.Config.Get()
	token := strings.TrimSpace(cfg.TgToken)
	chatID := strings.TrimSpace(cfg.TgChatID)
	if token == "" {
		return false, "未配置 Telegram Bot Token"
	}
	if chatID == "" {
		return false, "未配置 Telegram Chat ID"
	}
	client := &http.Client{Timeout: 10 * time.Second}

	// 1) getMe 验 token
	resp, err := client.Get("https://api.telegram.org/bot" + token + "/getMe")
	if err != nil {
		return false, "无法连接 Telegram API: " + err.Error()
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var r1 map[string]interface{}
	_ = json.Unmarshal(body, &r1)
	if ok, _ := r1["ok"].(bool); !ok {
		desc, _ := r1["description"].(string)
		if desc == "" {
			desc = "未知错误"
		}
		return false, "Telegram Token 无效: " + desc
	}

	// 2) getChat 验 chat_id (bot 是否能访问这个 chat)
	resp2, err := client.Get("https://api.telegram.org/bot" + token + "/getChat?chat_id=" + chatID)
	if err != nil {
		return false, "无法连接 Telegram API: " + err.Error()
	}
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	var r2 map[string]interface{}
	_ = json.Unmarshal(body2, &r2)
	if ok, _ := r2["ok"].(bool); !ok {
		desc, _ := r2["description"].(string)
		if desc == "" {
			desc = "未知错误"
		}
		return false, "Telegram Chat ID 不可达: " + desc + " (请先给 bot 发一条消息)"
	}
	return true, ""
}

func SendMessage(state *app.State, message string, replyMarkup map[string]interface{}) bool {
	_, ok := SendToChat(state, state.Config.Get().TgChatID, message, replyMarkup)
	return ok
}

// SendToChat 发到指定 chat，返回 Telegram message_id（用于随后 EditMessage）。
func SendToChat(state *app.State, chatID interface{}, message string, replyMarkup map[string]interface{}) (int64, bool) {
	cfg := state.Config.Get()
	if cfg.TgToken == "" {
		state.Logger.Warn("Telegram消息未发送: Bot Token未在config中设置", "")
		return 0, false
	}
	cid := strings.TrimSpace(idToString(chatID))
	if cid == "" {
		state.Logger.Warn("Telegram消息未发送: Chat ID 为空", "")
		return 0, false
	}
	payload := map[string]interface{}{
		"chat_id": cid,
		"text":    message,
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, "https://api.telegram.org/bot"+cfg.TgToken+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		state.Logger.Error("发送Telegram消息时发生未预期错误: "+err.Error(), "")
		return 0, false
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		state.Logger.Error("发送Telegram消息时发生网络错误: "+err.Error(), "")
		return 0, false
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		state.Logger.Error(fmt.Sprintf("发送消息到Telegram失败: 状态码=%d, 响应=%s", resp.StatusCode, string(respBody)), "")
		return 0, false
	}
	var parsed struct {
		OK     bool `json:"ok"`
		Result struct {
			MessageID int64 `json:"message_id"`
		} `json:"result"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	return parsed.Result.MessageID, parsed.OK || resp.StatusCode == http.StatusOK
}

// EditMessage 原地更新一条 Bot 消息（进度闭环）。
func EditMessage(state *app.State, chatID interface{}, messageID int64, text string, replyMarkup map[string]interface{}) bool {
	cfg := state.Config.Get()
	if cfg.TgToken == "" || messageID == 0 {
		return false
	}
	cid := strings.TrimSpace(idToString(chatID))
	if cid == "" {
		return false
	}
	payload := map[string]interface{}{
		"chat_id":    cid,
		"message_id": messageID,
		"text":       text,
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, "https://api.telegram.org/bot"+cfg.TgToken+"/editMessageText", bytes.NewReader(body))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		state.Logger.Warn("editMessageText 网络错误: "+err.Error(), "telegram")
		return false
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		state.Logger.Debug("editMessageText 失败: "+string(respBody), "telegram")
		return false
	}
	return true
}

func EmptyInlineKeyboard() map[string]interface{} {
	return map[string]interface{}{"inline_keyboard": [][]map[string]string{}}
}

// DeleteWebhook 去掉 Bot 上已注册的 Webhook，否则 getUpdates 会 409。
func DeleteWebhook(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	payload, _ := json.Marshal(map[string]any{"drop_pending_updates": false})
	req, err := http.NewRequest(http.MethodPost, "https://api.telegram.org/bot"+token+"/deleteWebhook", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(body, &result)
	if ok, _ := result["ok"].(bool); ok {
		return nil
	}
	desc, _ := result["description"].(string)
	if desc == "" {
		desc = string(body)
	}
	return fmt.Errorf("%s", desc)
}

// AnswerCallback 应答 callback_query
func AnswerCallback(state *app.State, callbackQueryID, text string, showAlert bool) {
	cfg := state.Config.Get()
	if cfg.TgToken == "" {
		return
	}
	payload := map[string]interface{}{
		"callback_query_id": callbackQueryID,
		"text":              text,
		"show_alert":        showAlert,
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest(http.MethodPost,
		"https://api.telegram.org/bot"+cfg.TgToken+"/answerCallbackQuery",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// SendReply 回复指定消息
func SendReply(state *app.State, chatID interface{}, text string, replyToMessageID int64) {
	cfg := state.Config.Get()
	if cfg.TgToken == "" {
		return
	}
	payload := map[string]interface{}{
		"chat_id":             chatID,
		"text":                text,
		"reply_to_message_id": replyToMessageID,
	}
	body, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest(http.MethodPost,
		"https://api.telegram.org/bot"+cfg.TgToken+"/sendMessage",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

type OrderInfo struct {
	PlanCode   string
	Datacenter string
	Quantity   int
	Options    []string
}

// 格式: plancode [datacenter] [quantity] [options(逗号分隔)]
func ParseOrderMessage(text string) *OrderInfo {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return nil
	}
	result := &OrderInfo{
		PlanCode: parts[0],
		Quantity: 1,
	}
	remaining := []string{}
	if len(parts) > 1 {
		remaining = parts[1:]
	}
	if len(remaining) == 0 {
		return result
	}

	// 找包含逗号的部分 = options
	optionsStart := -1
	for i, p := range remaining {
		if strings.Contains(p, ",") {
			optionsStart = i
			break
		}
	}
	if optionsStart >= 0 {
		optsText := strings.Join(remaining[optionsStart:], " ")
		for _, o := range strings.Split(optsText, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				result.Options = append(result.Options, o)
			}
		}
		remaining = remaining[:optionsStart]
	}

	switch len(remaining) {
	case 1:
		p := remaining[0]
		if n, ok := parsePositiveInt(p); ok {
			result.Quantity = n
		} else if len(p) >= 3 && len(p) <= 4 && isAlpha(p) {
			result.Datacenter = strings.ToLower(p)
		}
	case 2:
		p1, p2 := remaining[0], remaining[1]
		if len(p1) >= 3 && len(p1) <= 4 && isAlpha(p1) {
			result.Datacenter = strings.ToLower(p1)
			if n, ok := parsePositiveInt(p2); ok {
				result.Quantity = n
			}
		} else if n, ok := parsePositiveInt(p1); ok {
			result.Quantity = n
			if len(p2) >= 3 && len(p2) <= 4 && isAlpha(p2) {
				result.Datacenter = strings.ToLower(p2)
			}
		}
	}
	return result
}

// parsePositiveInt 只接受纯十进制 ASCII 数字字符串，
// 不接受 "-1" / "+5" / " 3" 等带符号或空白的版本（strconv.Atoi 会通过）。
func parsePositiveInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

func isAlpha(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return len(s) > 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SetMyCommands 向 Telegram 注册 Bot 命令菜单（/buy /stock /tasks 等）。
// 成功返回空串；失败返回错误描述。
func SetMyCommands(state *app.State) string {
	cfg := state.Config.Get()
	if strings.TrimSpace(cfg.TgToken) == "" {
		return "未配置 Telegram Bot Token"
	}
	type botCmd struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}
	commands := []botCmd{
		{Command: "start", Description: "显示帮助与可用操作"},
		{Command: "help", Description: "命令帮助"},
		{Command: "buy", Description: "快速下单或抢购 [planCode] [dc]"},
		{Command: "stock", Description: "查询库存并开抢: /stock <planCode>"},
		{Command: "tasks", Description: "查看当前抢购任务队列"},
		{Command: "accounts", Description: "查看与切换 OVH 账号"},
		{Command: "queue", Description: "加入排队队列: /queue <planCode> [dc]"},
		{Command: "monitor", Description: "添加库存监控: /monitor <planCode>"},
		{Command: "price", Description: "查询价格: /price <planCode> <dc>"},
	}
	payload, _ := json.Marshal(map[string]interface{}{"commands": commands})
	apiURL := "https://api.telegram.org/bot" + cfg.TgToken + "/setMyCommands"
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(body, &result)
	if ok, _ := result["ok"].(bool); ok {
		state.Logger.Info("Telegram setMyCommands 成功", "telegram")
		return ""
	}
	desc, _ := result["description"].(string)
	if desc == "" {
		desc = string(body)
	}
	return desc
}
