package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// TelegramStatus GET /api/telegram/status
func TelegramStatus(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"polling": telegram.SnapshotPoller(),
		})
	}
}

// ProcessTelegramUpdate 轮询入站的统一入口：幂等 → 按钮回调 / 文本命令。
func ProcessTelegramUpdate(state *app.State, mon *monitor.Monitor, data map[string]interface{}) {
	if data == nil {
		return
	}
	if updateID := telegram.ParseUpdateID(data["update_id"]); updateID > 0 && state.DB != nil {
		claimed, err := state.DB.TryClaimTelegramUpdate(updateID)
		if err != nil {
			state.Logger.Warn("update_id 幂等写入失败: "+err.Error(), "telegram")
		} else if !claimed {
			state.Logger.Info(fmt.Sprintf("忽略重复投递的 update_id=%d", updateID), "telegram")
			return
		}
		if updateID%50 == 0 {
			before := float64(time.Now().Add(-time.Duration(telegram.UpdateIDRetentionDays) * 24 * time.Hour).Unix())
			if n, err := state.DB.CleanupTelegramUpdates(before); err == nil && n > 0 {
				state.Logger.Debug(fmt.Sprintf("已清理 %d 条过期 update_id", n), "telegram")
			}
		}
	}
	if cb, ok := data["callback_query"].(map[string]interface{}); ok {
		handleTelegramCallback(state, mon, cb)
		return
	}
	if msg, ok := data["message"].(map[string]interface{}); ok {
		handleTelegramMessage(state, mon, msg)
	}
}

// handleTelegramCallback 处理「一键下单」按钮回调。
func handleTelegramCallback(state *app.State, mon *monitor.Monitor, cb map[string]interface{}) {
	cbData, _ := cb["data"].(string)
	message, _ := cb["message"].(map[string]interface{})
	chatID := getNested(message, "chat", "id")
	messageID, _ := getNumOrFloat(message["message_id"])
	fromUser, _ := cb["from"].(map[string]interface{})
	userID, _ := getNumOrFloat(fromUser["id"])
	state.Logger.Info(fmt.Sprintf("收到Telegram回调: user_id=%v, callback_data=%s...", userID, truncate(cbData, 50)), "telegram")

	// 4) 发送者授权：只有配置的那个 chat 能下单
	if !telegram.IsAuthorizedActor(state, chatID, fromUser["id"]) {
		state.Logger.Warn(fmt.Sprintf("拒绝未授权的 Telegram 回调: chat_id=%v, user_id=%v", chatID, userID), "telegram")
		telegram.AnswerCallback(state, fmt.Sprintf("%v", cb["id"]), "无权限", true)
		return
	}

	// 5) 频率限制
	rateKey := telegram.ChatIDString(chatID)
	if rateKey == "" {
		rateKey = telegram.ChatIDString(fromUser["id"])
	}
	if !telegram.AllowRate(rateKey) {
		telegram.AnswerCallback(state, fmt.Sprintf("%v", cb["id"]), "操作过于频繁，请稍后再试", true)
		return
	}

	cbID := fmt.Sprintf("%v", cb["id"])
	msgID := int64(messageID)
	if handleInlineCallback(state, mon, cbID, chatID, msgID, cbData) {
		return
	}

	callbackObj, ok := decodeCallbackData(state, cbData)
	if !ok {
		telegram.AnswerCallback(state, cbID, "按钮数据无效", true)
		return
	}

	action := strOr(callbackObj, "a", "action")
	buttonID := strOr(callbackObj, "u", "uuid")
	switch action {
	case "sniper", "queue_all":
		telegram.AnswerCallback(state, cbID, "已收到", false)
		enqueueFromNotifyButton(state, mon, cbID, chatID, msgID, buttonID, action, "", false)
		return
	case "pick_acc":
		telegram.AnswerCallback(state, cbID, "选择账户", false)
		rememberShort(buttonID)
		showAccountPicker(state, chatID, msgID, buttonID)
		return
	case "add_to_queue":
		// 旧版按机房一键下单
	default:
		state.Logger.Warn("未知的action: "+action, "telegram")
		telegram.AnswerCallback(state, cbID, "未知操作", true)
		return
	}

	planCode := strOr(callbackObj, "p", "planCode")
	dc := strOr(callbackObj, "d", "datacenter")
	// btnAccountID:发通知时记下的「触发订阅所用账户」。planCode 是分区的,
	// 用它下单才不会把欧区机型落到美区账户上。空 = 老按钮/无账户维度 → 退回默认账户。
	btnAccountID := ""
	var options []string
	if optsRaw, ok := callbackObj["o"]; ok {
		options = toStringSlice(optsRaw)
	} else if optsRaw, ok := callbackObj["options"]; ok {
		options = toStringSlice(optsRaw)
	}

	claimed := false // 是否占用了 DB 里的一次性按钮（失败要回滚）
	if buttonID != "" {
		row, ok, err := state.DB.ClaimTelegramButton(buttonID)
		if err != nil {
			state.Logger.Error("认领一键下单按钮失败: "+err.Error(), "telegram")
			telegram.AnswerCallback(state, fmt.Sprintf("%v", cb["id"]), "认领按钮失败", true)
			return
		}
		if ok {
			// 按钮过期（默认 24h）→ 归还并拒绝，避免用很久以前的库存信息下单
			if time.Since(time.Unix(int64(row.CreatedAt), 0)) > telegram.ButtonTTL {
				_ = state.DB.UnclaimTelegramButton(buttonID)
				state.Logger.Warn("一键下单按钮已过期: "+buttonID, "telegram")
				telegram.AnswerCallback(state, fmt.Sprintf("%v", cb["id"]), "该按钮已过期，请等待新的上架通知", true)
				return
			}
			claimed = true
			planCode = row.PlanCode
			dc = row.Datacenter
			options = db.ParseTelegramButtonOptions(row.Options)
			btnAccountID = strings.TrimSpace(row.AccountID)
			state.Logger.Info(fmt.Sprintf("✅ 按钮已认领: id=%s, %s@%s, options=%v, account=%s",
				buttonID, planCode, dc, options, btnAccountID), "telegram")
		} else {
			// 认领失败：要么已经点过（重放），要么这条按钮根本不存在
			if _, exists, _ := state.DB.GetTelegramButton(buttonID); exists {
				state.Logger.Warn("一键下单按钮已被使用过，拒绝重复下单: "+buttonID, "telegram")
				telegram.AnswerCallback(state, fmt.Sprintf("%v", cb["id"]), "该按钮已使用过", true)
				return
			}
			// 库里没有 → 退回内存缓存（升级前发出、只存在内存里的老按钮）
			if cached := mon.MessageUUIDCacheLookup(buttonID); cached != nil {
				planCode = cached.PlanCode
				dc = cached.Datacenter
				options = cached.Options
				state.Logger.Info("从内存缓存恢复按钮配置（旧按钮）: "+buttonID, "telegram")
			} else {
				state.Logger.Warn("按钮 UUID 不存在: "+buttonID, "telegram")
			}
		}
	}

	if planCode == "" || dc == "" {
		if claimed {
			_ = state.DB.UnclaimTelegramButton(buttonID)
		}
		telegram.AnswerCallback(state, fmt.Sprintf("%v", cb["id"]), "缺少型号或机房", true)
		return
	}
	if len(options) == 0 {
		if cachedOpts := mon.OptionsCacheLookup(planCode + "|" + dc); len(cachedOpts) > 0 {
			options = cachedOpts
			state.Logger.Info("✅ 从缓存恢复 options: "+planCode+"|"+dc, "telegram")
		}
	}

	// 入队账户:优先用按钮里记下的那个(monitor 发通知时写入的触发订阅账户),
	// 它和查到这批库存的那个大区一致;按钮没记账户(老按钮/单账户)才退回默认账户。
	// 留空不入队 —— 会让下游 history / 账户 chip 全都对不上号。
	//
	// 一个账户都没有时不能"留空照样入队":这一单永远下不出去，
	// 用户却收到一句"已添加到抢购队列"，等于把失败藏到几十次重试之后。
	acc, hasAcc := state.FindAccount(btnAccountID)
	if !hasAcc && btnAccountID != "" {
		// 按钮里的账户已被删除 —— 不能直接失败(用户还有别的账户可下),
		// 但必须落一条 Warn:退回默认账户很可能就是跨区下错单的那一刻。
		state.Logger.Warn("按钮记录的账户已不存在，退回默认账户: "+btnAccountID, "telegram")
		btnAccountID = ""
		acc, hasAcc = state.FindAccount("")
	}
	if !hasAcc {
		if claimed {
			_ = state.DB.UnclaimTelegramButton(buttonID)
		}
		state.Logger.Warn("Telegram 一键下单被拒绝：系统里没有任何 OVH 账户", "telegram")
		telegram.AnswerCallback(state, fmt.Sprintf("%v", cb["id"]), "未配置 OVH 账户", true)
		telegram.SendReply(state, chatID, "❌ 未配置任何 OVH 账户，无法下单。请先在控制台添加账户。", int64(messageID))
		return
	}
	accountID := acc.ID
	// planCode 是分区的：欧区 planCode 落到美区账户上，OVH 只会返回空库存而不报错。
	// 按钮带账户时这里就是发通知时那个订阅的账户；退回默认账户的情况仍可能选错，
	// 所以把"这一单会用哪个账户、哪个子公司/大区"明确写进日志和回复，
	// 让用户在真正扣款前就能看出账户选错了。
	accSub := strings.ToUpper(strings.TrimSpace(acc.Zone))
	if accSub == "" {
		accSub = ovh.DefaultSubsidiaryForEndpoint(acc.Endpoint)
	}
	accLabel := fmt.Sprintf("%s（子公司 %s / %s 区）", acc.Name, accSub, ovh.SubsidiaryRegion(accSub))
	if btnAccountID == "" {
		// 明示"这是兜底账户"而不是通知里那个订阅账户，用户才知道要核对
		accLabel += "［默认账户］"
	}
	item := types.QueueItem{
		ID:            uuid.NewString(),
		AccountID:     accountID,
		PlanCode:      planCode,
		Datacenter:    dc,
		Options:       options,
		Status:        "running",
		CreatedAt:     types.NowISO(),
		UpdatedAt:     types.NowISO(),
		RetryInterval: 30,
		RetryCount:    0,
		LastCheckTime: 0,
		FromTelegram:  true,
	}
	state.QueueMu.Lock()
	state.Queue = append(state.Queue, item)
	state.QueueMu.Unlock()
	if err := state.SaveQueue(); err != nil {
		// 落库失败 → 归还按钮，让用户可以重试
		state.Logger.Error("Telegram 入队后保存失败: "+err.Error(), "telegram")
		if claimed {
			_ = state.DB.UnclaimTelegramButton(buttonID)
		}
	}

	optsStr := strings.Join(options, ", ")
	if optsStr == "" {
		optsStr = "无（默认配置）"
	}
	state.Logger.Info(fmt.Sprintf("Telegram用户 %v 通过按钮添加到队列: %s@%s, 配置选项: %s, 账户: %s",
		userID, planCode, dc, optsStr, accLabel), "telegram")
	confirmMsg := fmt.Sprintf("✅ 已添加到抢购队列！\n\n型号: %s\n机房: %s\n配置: %s\n账户: %s\n\n系统将自动尝试下单。",
		planCode, strings.ToUpper(dc), optsStr, accLabel)
	telegram.AnswerCallback(state, fmt.Sprintf("%v", cb["id"]), "已添加到队列！", false)
	telegram.SendReply(state, chatID, confirmMsg, int64(messageID))
}

// handleTelegramMessage 处理斜杠命令和文本下单消息。
func handleTelegramMessage(state *app.State, mon *monitor.Monitor, msg map[string]interface{}) {
	text, _ := msg["text"].(string)
	text = strings.TrimSpace(text)
	chatID := getNested(msg, "chat", "id")
	messageID, _ := getNumOrFloat(msg["message_id"])
	fromUser, _ := msg["from"].(map[string]interface{})
	userID, _ := getNumOrFloat(fromUser["id"])
	username, _ := fromUser["username"].(string)
	if username == "" {
		username = "未知用户"
	}
	state.Logger.Info(fmt.Sprintf("收到Telegram普通消息: user_id=%v, username=%s, text=%s",
		userID, username, truncate(text, 100)), "telegram")

	rateKey := telegram.ChatIDString(chatID)
	if rateKey == "" {
		rateKey = telegram.ChatIDString(fromUser["id"])
	}
	if !telegram.AllowRate(rateKey) {
		telegram.SendReply(state, chatID, "⚠️ 操作过于频繁，请稍后再试", int64(messageID))
		return
	}

	handleTelegramText(state, mon, text, chatID, userID, messageID)
}

// decodeCallbackData 解析 callback_data，支持 "b64:" 前缀的 base64 包装和裸 JSON 两种。
func decodeCallbackData(state *app.State, cbData string) (map[string]interface{}, bool) {
	payload := []byte(cbData)
	if strings.HasPrefix(cbData, "b64:") {
		base64Part := cbData[4:]
		if missing := len(base64Part) % 4; missing != 0 {
			base64Part += strings.Repeat("=", 4-missing)
		}
		decoded, err := base64.StdEncoding.DecodeString(base64Part)
		if err != nil {
			state.Logger.Warn(fmt.Sprintf("base64解码失败（可能是数据被截断）: %s, base64_len=%d", err.Error(), len(cbData[4:])), "telegram")
			return nil, false
		}
		payload = decoded
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(payload, &obj); err != nil {
		state.Logger.Error("解析callback_data JSON失败: "+err.Error()+", data="+truncate(cbData, 100), "telegram")
		return nil, false
	}
	return obj, true
}

func getNested(m map[string]interface{}, keys ...string) interface{} {
	var cur interface{} = m
	for _, k := range keys {
		mm, ok := cur.(map[string]interface{})
		if !ok {
			return nil
		}
		cur = mm[k]
	}
	return cur
}

func getNumOrFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func strOr(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

func toStringSlice(v interface{}) []string {
	out := []string{}
	switch x := v.(type) {
	case []interface{}:
		for _, e := range x {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, x...)
	}
	return out
}
