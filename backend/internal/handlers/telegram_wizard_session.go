package handlers

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/telegram"
)

type dcPickerSession struct {
	PlanCode    string
	Mode        string
	Selected    map[string]bool
	Options     []string
	ConfigLabel string
	UpdatedAt   time.Time
}

var (
	shortMu        sync.Mutex
	shortToID      = map[string]string{}
	pickerMu       sync.Mutex
	pickerSessions = map[string]*dcPickerSession{}
	narrowMu       sync.Mutex
	narrowOffers   = map[string]*narrowOffer{}
	cfgMu          sync.Mutex
	cfgOffers      = map[string]*cfgOffer{}
)

const (
	narrowTTL           = 10 * time.Minute
	narrowMaxButtons    = 8
	maxShortMemoryItems = 5000
)

// narrowOffer 快捷 /monitor 建完后「一键改窄配置」的待选列表。
type narrowOffer struct {
	PlanCode string
	Configs  []wizardConfigChoice
	Expires  time.Time
}

// cfgOffer 冻住配置选择器的选项。
type cfgOffer struct {
	Mode      string
	PlanCode  string
	Configs   []wizardConfigChoice
	TargetDCs []string
	Quantity  int
	Expires   time.Time
}

type wizardConfigChoice struct {
	Label   string
	Options []string
	InStock int
}

// tgBtnLabel Telegram 按钮文字上限 64 个字符,超了 API 不报错,按钮发出去点了没反应。
func tgBtnLabel(s string) string {
	r := []rune(s)
	if len(r) <= 64 {
		return s
	}
	return string(r[:61]) + "..."
}

func pickerKey(chatID interface{}, messageID int64) string {
	return fmt.Sprintf("%v_%d", chatID, messageID)
}

func getPickerSession(chatID interface{}, messageID int64, mode, planCode string) *dcPickerSession {
	key := pickerKey(chatID, messageID)
	pickerMu.Lock()
	defer pickerMu.Unlock()

	// 清理 30 分钟未更新的废弃会话，防止内存泄漏
	if len(pickerSessions) > 100 {
		cutoff := time.Now().Add(-30 * time.Minute)
		for k, s := range pickerSessions {
			if s.UpdatedAt.Before(cutoff) {
				delete(pickerSessions, k)
			}
		}
	}

	sess, ok := pickerSessions[key]
	if !ok || sess.PlanCode != planCode || sess.Mode != mode {
		sess = &dcPickerSession{
			PlanCode:  planCode,
			Mode:      mode,
			Selected:  map[string]bool{},
			UpdatedAt: time.Now(),
		}
		pickerSessions[key] = sess
	}
	sess.UpdatedAt = time.Now()
	return sess
}

func clearPickerSession(chatID interface{}, messageID int64) {
	key := pickerKey(chatID, messageID)
	pickerMu.Lock()
	delete(pickerSessions, key)
	pickerMu.Unlock()
}

func migratePickerSession(chatID interface{}, fromID, toID int64, mode, planCode string) {
	if toID == 0 || fromID == toID {
		return
	}
	from, to := pickerKey(chatID, fromID), pickerKey(chatID, toID)
	pickerMu.Lock()
	defer pickerMu.Unlock()
	sess, ok := pickerSessions[from]
	if !ok || sess.PlanCode != planCode || sess.Mode != mode {
		return
	}
	pickerSessions[to] = sess
	if fromID == 0 {
		delete(pickerSessions, from)
	}
}

func rememberShort(state *app.State, full string, category ...string) string {
	full = strings.TrimSpace(full)
	if full == "" {
		return ""
	}
	cat := ""
	if len(category) > 0 {
		cat = category[0]
	}

	// 优先在 SQLite 检查是否已有映射，避免重复生成不同 short ID
	if state != nil && state.TelegramRepo != nil {
		if existingShort, ok, _ := state.TelegramRepo.FindShortIDByFull(full); ok && existingShort != "" {
			shortMu.Lock()
			shortToID[existingShort] = full
			shortMu.Unlock()
			return existingShort
		}
	}

	cleaned := strings.ReplaceAll(full, "-", "")
	// 基础长度 8 位，若冲突则逐步扩展至 10、12 位
	targetLen := 8
	if targetLen > len(cleaned) {
		targetLen = len(cleaned)
	}
	s := cleaned[:targetLen]

	shortMu.Lock()
	// 防哈希碰撞自增长度
	for {
		existing, exists := shortToID[s]
		if !exists || existing == full {
			break
		}
		if targetLen < len(cleaned) {
			targetLen += 2
			if targetLen > len(cleaned) {
				targetLen = len(cleaned)
			}
			s = cleaned[:targetLen]
		} else {
			break
		}
	}

	// 限制内存缓存上限，防止无界增长
	if len(shortToID) > maxShortMemoryItems {
		count := 0
		for k := range shortToID {
			delete(shortToID, k)
			count++
			if count >= maxShortMemoryItems/5 {
				break
			}
		}
	}
	shortToID[s] = full
	shortMu.Unlock()

	// 持久化到 SQLite
	if state != nil && state.TelegramRepo != nil {
		_ = state.TelegramRepo.UpsertShortID(s, full, cat)
	}
	return s
}

func resolveShort(state *app.State, s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// L1: 内存快速查询
	shortMu.Lock()
	if v, ok := shortToID[s]; ok && v != "" {
		shortMu.Unlock()
		return v
	}
	shortMu.Unlock()

	// L2: SQLite 持久化查询（跨进程重启后仍有效）
	if state != nil && state.TelegramRepo != nil {
		if full, ok, err := state.TelegramRepo.GetShortID(s); err == nil && ok && full != "" {
			shortMu.Lock()
			shortToID[s] = full
			shortMu.Unlock()
			return full
		}
	}

	// L3: 智能业务前缀与全量匹配兜底
	// 3.1 账户 ID 前缀匹配
	if state != nil {
		state.AccountsMu.RLock()
		for _, a := range state.Accounts {
			cleanAcc := strings.ReplaceAll(a.ID, "-", "")
			cleanS := strings.ReplaceAll(s, "-", "")
			if a.ID == s || strings.HasPrefix(a.ID, s) || strings.HasPrefix(cleanAcc, cleanS) {
				state.AccountsMu.RUnlock()
				shortMu.Lock()
				shortToID[s] = a.ID
				shortMu.Unlock()
				return a.ID
			}
		}
		state.AccountsMu.RUnlock()
	}

	// 3.2 队列任务 ID 前缀匹配
	if state != nil {
		state.QueueMu.Lock()
		for _, it := range state.Queue {
			cleanTask := strings.ReplaceAll(it.ID, "-", "")
			cleanS := strings.ReplaceAll(s, "-", "")
			if it.ID == s || strings.HasPrefix(it.ID, s) || strings.HasPrefix(cleanTask, cleanS) {
				state.QueueMu.Unlock()
				shortMu.Lock()
				shortToID[s] = it.ID
				shortMu.Unlock()
				return it.ID
			}
		}
		state.QueueMu.Unlock()
	}

	return s
}

func putNarrowOffer(planCode string, configs []wizardConfigChoice) string {
	tok := fmt.Sprintf("%x", time.Now().UnixNano())
	narrowMu.Lock()
	now := time.Now()
	for k, v := range narrowOffers {
		if now.After(v.Expires) {
			delete(narrowOffers, k)
		}
	}
	narrowOffers[tok] = &narrowOffer{
		PlanCode: planCode,
		Configs:  configs,
		Expires:  now.Add(narrowTTL),
	}
	narrowMu.Unlock()
	return tok
}

func takeNarrowOffer(tok string) (*narrowOffer, bool) {
	narrowMu.Lock()
	defer narrowMu.Unlock()
	o, ok := narrowOffers[tok]
	if !ok || time.Now().After(o.Expires) {
		delete(narrowOffers, tok)
		return nil, false
	}
	delete(narrowOffers, tok)
	return o, true
}

func offerNarrowConfig(state *app.State, chatID interface{}, _ int64, planCode string) bool {
	configs := enumerateWizardConfigs(state, planCode, "")
	if len(configs) <= 1 {
		return false
	}
	tok := putNarrowOffer(planCode, configs)
	var btns []map[string]string
	for i, c := range configs {
		if i >= narrowMaxButtons {
			break
		}
		stockBadge := "[🔴 缺货]"
		if c.InStock > 0 {
			stockBadge = fmt.Sprintf("[🟢 %d机房有货]", c.InStock)
		}
		btnLabel := tgBtnLabel(fmt.Sprintf("⚡ %s  %s", c.Label, stockBadge))
		btns = append(btns, telegram.CallbackButton(btnLabel, fmt.Sprintf("i:mon:n:%s:%d", tok, i)))
	}
	rows := telegram.ChunkButtons(btns, 1)
	rows = append(rows, []map[string]string{
		telegram.CallbackButton("🌐 保持监控全部硬件配置", fmt.Sprintf("i:mon:n:%s:all", tok)),
	})
	var b strings.Builder
	b.WriteString(fmt.Sprintf("🔧 规格细化 · %s\n", planCode))
	b.WriteString(telegram.CardDivider + "\n")
	b.WriteString(fmt.Sprintf("检测到该型号存在 %d 套物理硬件规格，当前默认监控全部配置。\n", len(configs)))
	b.WriteString("若您只想锁定特定硬件配置（避免其他配置放货时误触发），请点击下方直接锁定：")
	text := b.String()
	_, _ = telegram.SendToChat(state, chatID, text, telegram.InlineKeyboard(rows))
	return true
}

func putCfgOffer(o *cfgOffer) string {
	tok := fmt.Sprintf("%x", time.Now().UnixNano())
	o.Expires = time.Now().Add(narrowTTL)
	cfgMu.Lock()
	now := time.Now()
	for k, v := range cfgOffers {
		if now.After(v.Expires) {
			delete(cfgOffers, k)
		}
	}
	cfgOffers[tok] = o
	cfgMu.Unlock()
	return tok
}

func getCfgOffer(tok string) (*cfgOffer, bool) {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	o, ok := cfgOffers[tok]
	if !ok || time.Now().After(o.Expires) {
		delete(cfgOffers, tok)
		return nil, false
	}
	return o, true
}
