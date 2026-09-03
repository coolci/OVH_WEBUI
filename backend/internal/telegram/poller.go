package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/app"
)

// PollerStatus 轮询入站的运行快照，给设置页 / Telegram 下单页看。
type PollerStatus struct {
	Running      bool   `json:"running"`
	Configured   bool   `json:"configured"`
	BotUsername  string `json:"botUsername,omitempty"`
	LastError    string `json:"lastError,omitempty"`
	LastUpdateAt string `json:"lastUpdateAt,omitempty"`
	Offset       int64  `json:"offset"`
}

var (
	pollerMu      sync.Mutex
	pollerStatus  PollerStatus
	pollerKick    chan struct{}
	pollerStarted bool
)

// SnapshotPoller 返回轮询状态副本。
func SnapshotPoller() PollerStatus {
	pollerMu.Lock()
	defer pollerMu.Unlock()
	return pollerStatus
}

func patchPoller(fn func(*PollerStatus)) {
	pollerMu.Lock()
	fn(&pollerStatus)
	pollerMu.Unlock()
}

// NotifyPollerConfigChanged Token / Chat ID 保存后打断当前 getUpdates，立刻重读配置。
func NotifyPollerConfigChanged() {
	pollerMu.Lock()
	ch := pollerKick
	pollerMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

// StartPoller 启动唯一的 getUpdates 循环。重复调用是空操作。
// onUpdate 在 handlers 里接到现有的命令 / 一键下单处理。
func StartPoller(state *app.State, onUpdate func(map[string]interface{})) {
	if onUpdate == nil {
		return
	}
	pollerMu.Lock()
	if pollerStarted {
		pollerMu.Unlock()
		return
	}
	pollerStarted = true
	pollerKick = make(chan struct{}, 1)
	kick := pollerKick
	pollerMu.Unlock()
	go runPoller(state, onUpdate, kick)
}

func runPoller(state *app.State, onUpdate func(map[string]interface{}), kick <-chan struct{}) {
	client := &http.Client{Timeout: 40 * time.Second}
	var offset int64
	var lastToken string

	for {
		cfg := state.Config.Get()
		token := strings.TrimSpace(cfg.TgToken)
		patchPoller(func(s *PollerStatus) { s.Configured = token != "" })

		if token == "" {
			patchPoller(func(s *PollerStatus) {
				s.Running = false
				s.BotUsername = ""
				s.LastError = ""
			})
			lastToken = ""
			select {
			case <-kick:
			case <-time.After(3 * time.Second):
			}
			continue
		}

		if token != lastToken {
			if lastToken != "" {
				_ = DeleteWebhook(lastToken)
			}
			if err := DeleteWebhook(token); err != nil {
				state.Logger.Warn("清除 Telegram Webhook 失败（轮询仍会继续）: "+err.Error(), "telegram")
			} else {
				state.Logger.Info("已清除 Telegram Webhook，改用 getUpdates 轮询入站", "telegram")
			}
			offset = 0
			lastToken = token
			if u, err := fetchBotUsername(client, token); err == nil {
				patchPoller(func(s *PollerStatus) { s.BotUsername = u })
				state.Logger.Info("Telegram Bot 已连接: @"+u, "telegram")
			}
			if msg := SetMyCommands(state); msg != "" {
				state.Logger.Warn("注册 Bot 命令菜单失败: "+msg, "telegram")
			}
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			select {
			case <-kick:
				cancel()
			case <-done:
			}
		}()
		updates, err := fetchUpdates(ctx, client, token, offset)
		close(done)
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				continue
			}
			errText := err.Error()
			patchPoller(func(s *PollerStatus) {
				s.Running = false
				s.LastError = errText
			})
			state.Logger.Warn("getUpdates 失败: "+errText, "telegram")
			low := strings.ToLower(errText)
			if strings.Contains(low, "409") || strings.Contains(low, "webhook") {
				lastToken = ""
			}
			select {
			case <-kick:
			case <-time.After(3 * time.Second):
			}
			continue
		}

		patchPoller(func(s *PollerStatus) {
			s.Running = true
			s.LastError = ""
		})

		for _, u := range updates {
			id := ParseUpdateID(u["update_id"])
			func() {
				defer func() {
					if rec := recover(); rec != nil {
						state.Logger.Error(fmt.Sprintf("处理 Telegram update 崩溃: %v", rec), "telegram")
					}
				}()
				onUpdate(u)
			}()
			if id >= offset {
				offset = id + 1
			}
			patchPoller(func(s *PollerStatus) {
				s.Offset = offset
				s.LastUpdateAt = time.Now().UTC().Format(time.RFC3339)
			})
		}
	}
}

func fetchUpdates(ctx context.Context, client *http.Client, token string, offset int64) ([]map[string]interface{}, error) {
	q := url.Values{}
	q.Set("timeout", "25")
	q.Set("allowed_updates", `["message","callback_query"]`)
	if offset > 0 {
		q.Set("offset", strconv.FormatInt(offset, 10))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.telegram.org/bot"+token+"/getUpdates?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	ok, desc, updates, err := parseGetUpdatesBody(body)
	if err != nil {
		return nil, err
	}
	if !ok {
		if desc == "" {
			desc = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("%s", desc)
	}
	return updates, nil
}

func parseGetUpdatesBody(body []byte) (ok bool, desc string, updates []map[string]interface{}, err error) {
	var raw struct {
		OK          bool              `json:"ok"`
		Description string            `json:"description"`
		Result      []json.RawMessage `json:"result"`
	}
	if err = json.Unmarshal(body, &raw); err != nil {
		return false, "", nil, err
	}
	out := make([]map[string]interface{}, 0, len(raw.Result))
	for _, item := range raw.Result {
		var u map[string]interface{}
		if e := json.Unmarshal(item, &u); e != nil {
			continue
		}
		out = append(out, u)
	}
	return raw.OK, raw.Description, out, nil
}

func fetchBotUsername(client *http.Client, token string) (string, error) {
	resp, err := client.Get("https://api.telegram.org/bot" + token + "/getMe")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", err
	}
	if ok, _ := raw["ok"].(bool); !ok {
		desc, _ := raw["description"].(string)
		if desc == "" {
			desc = "getMe failed"
		}
		return "", fmt.Errorf("%s", desc)
	}
	result, _ := raw["result"].(map[string]interface{})
	u, _ := result["username"].(string)
	return strings.TrimSpace(u), nil
}

// ParseUpdateID 从 Telegram JSON 数字里取出 update_id。
func ParseUpdateID(v interface{}) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	case int64:
		return x
	case int:
		return int64(x)
	}
	return 0
}
