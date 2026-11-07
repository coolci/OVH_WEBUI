// Package notify 把通知发到所有已配置的通道。
//
// 为什么要有第二条通道:补货监控的全部价值就是"有货的那一刻你能收到消息"。
// 而在此之前,Telegram 是唯一通道,并且监控在 Telegram 校验失败时会**自动停止**——
// 也就是说 TG bot 被封、token 失效、机器连不上 api.telegram.org 这几种情况下,
// 用户不是"少收一条通知",而是整个监控停摆,且只有翻日志才知道。
//
// 现在只要还有一条通道可用,监控就继续跑;全部通道都不可用才停。
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/telegram"
)

// AllowLocalWebhookForTesting 仅在单元测试中临时允许回环地址
var AllowLocalWebhookForTesting = false

// IsPrivateOrLocalIP 检查 IP 是否为本地回环、RFC1918 私网、链路本地、云元数据地址
func IsPrivateOrLocalIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 127 { // 127.0.0.0/8
			return true
		}
		if ip4[0] == 10 { // 10.0.0.0/8
			return true
		}
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 { // 172.16.0.0/12
			return true
		}
		if ip4[0] == 192 && ip4[1] == 168 { // 192.168.0.0/16
			return true
		}
		if ip4[0] == 169 && ip4[1] == 254 { // 169.254.0.0/16 (链路本地 / 云厂商元数据 169.254.169.254)
			return true
		}
		if ip4[0] == 0 { // 0.0.0.0
			return true
		}
	} else {
		// IPv6 fc00::/7 (unique local)
		if len(ip) == 16 && (ip[0]&0xfe) == 0xfc {
			return true
		}
	}
	return false
}

// ValidateWebhookURL 校验 URL 是否合法且不属于内网 SSRF 风险目标
func ValidateWebhookURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("通知 Webhook 地址不合法,必须是完整的 http:// 或 https:// 地址")
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		if AllowLocalWebhookForTesting {
			return nil
		}
		return fmt.Errorf("通知 Webhook 地址禁止指向本地或内网地址 (localhost)")
	}
	if ip := net.ParseIP(host); ip != nil {
		if IsPrivateOrLocalIP(ip) {
			if AllowLocalWebhookForTesting && (ip.IsLoopback() || ip.Equal(net.IPv4(127, 0, 0, 1))) {
				return nil
			}
			return fmt.Errorf("通知 Webhook 地址禁止指向私有或内网 IP (%s)", host)
		}
		return nil
	}
	// 尝试域名解析预检
	ips, err := net.LookupIP(host)
	if err == nil {
		for _, ip := range ips {
			if IsPrivateOrLocalIP(ip) {
				if AllowLocalWebhookForTesting && (ip.IsLoopback() || ip.Equal(net.IPv4(127, 0, 0, 1))) {
					continue
				}
				return fmt.Errorf("通知 Webhook 域名解析为私有或内网 IP (%s -> %s)", host, ip.String())
			}
		}
	}
	return nil
}

// Channel 一条通知通道的状态
type Channel struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	OK         bool   `json:"ok"`
	Detail     string `json:"detail,omitempty"`
}

// webhookPayload 保留自建服务原有字段,同时支持飞书的 msg_type/content。
type webhookPayload struct {
	Text          interface{} `json:"text"`
	Message       string      `json:"message"`
	Title         string      `json:"title"`
	MsgType       string      `json:"msgtype"`
	FeishuMsgType string      `json:"msg_type"`
	FeishuContent struct {
		Text string `json:"text"`
	} `json:"content"`
	Content struct {
		Text string `json:"text"`
	} `json:"text_content"`
}

// Broadcast 把一条消息发到所有已配置的通道。
// 返回成功送达的通道数 —— 调用方据此判断"这条通知到底有没有发出去"。
//
// 注意 replyMarkup 只有 Telegram 支持(一键下单按钮),webhook 那边只能收到纯文本。
// 这是有意的:让按钮在 TG 上继续可用,同时保证 TG 挂掉时消息本身还能到达。
func Broadcast(state *app.State, message string, replyMarkup map[string]interface{}) int {
	delivered, _ := BroadcastWithStatus(state, message, replyMarkup)
	return delivered
}

// BroadcastWithStatus 返回本次实际发送结果,避免测试通知拿配置状态冒充送达状态。
func BroadcastWithStatus(state *app.State, message string, replyMarkup map[string]interface{}) (int, []Channel) {
	delivered := 0
	cfg := state.Config.Get()
	tg := Channel{Name: "Telegram", Configured: strings.TrimSpace(cfg.TgToken) != "" && strings.TrimSpace(cfg.TgChatID) != ""}
	wh := Channel{Name: "Webhook", Configured: strings.TrimSpace(cfg.NotifyWebhookURL) != ""}
	if tg.Configured {
		if telegram.SendMessage(state, message, replyMarkup) {
			delivered++
			tg.OK = true
		} else {
			tg.Detail = "发送失败,请检查 Telegram 配置及网络"
		}
	}
	if wh.Configured {
		if err := sendWebhook(strings.TrimSpace(cfg.NotifyWebhookURL), message); err != nil {
			wh.Detail = err.Error()
			state.Logger.Warn("Webhook 通知发送失败: "+err.Error(), "notify")
		} else {
			delivered++
			wh.OK = true
		}
	}
	if delivered == 0 {
		state.Logger.Error("这条通知一个通道都没送达,请检查 Telegram / Webhook 配置", "notify")
	}
	return delivered, []Channel{tg, wh}
}

// sendWebhook 发文本消息,同时检查 HTTP 状态和常见的业务状态码。
func sendWebhook(webhookURL, message string) error {
	if err := ValidateWebhookURL(webhookURL); err != nil {
		return err
	}
	target, err := url.Parse(webhookURL)
	if err != nil {
		return err
	}
	host := strings.ToLower(target.Hostname())
	feishu := host == "open.feishu.cn" || host == "open.larksuite.com"
	dingtalk := host == "oapi.dingtalk.com"
	var p webhookPayload
	p.Text = message
	p.Message = message
	p.Title = "OVH 控制台"
	p.MsgType = "text"
	p.Content.Text = message
	p.FeishuMsgType = "text"
	p.FeishuContent.Text = message
	// 钉钉的 text 必须是对象;自建服务保持原来的字符串字段。
	if dingtalk {
		p.Text = map[string]string{"content": message}
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	transport := http.DefaultTransport
	if tr, ok := transport.(*http.Transport); ok {
		cloned := tr.Clone()
		origDial := cloned.DialContext
		if origDial == nil {
			origDial = (&net.Dialer{Timeout: 5 * time.Second}).DialContext
		}
		cloned.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			h, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIP(ctx, "ip", h)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if IsPrivateOrLocalIP(ip) {
					if AllowLocalWebhookForTesting && (ip.IsLoopback() || ip.Equal(net.IPv4(127, 0, 0, 1))) {
						continue
					}
					return nil, fmt.Errorf("SSRF 拦截: 目标 IP %s 属于私有或本地网络", ip.String())
				}
			}
			// 直接连刚校验过的 IP,而不是把主机名再交给拨号器解析一次:
			// 两次解析之间 DNS 可以换答案(DNS rebinding),校验就形同虚设。
			// TLS 的 SNI/证书校验仍按 URL 里的主机名进行,不受影响。
			var lastErr error
			for _, ip := range ips {
				conn, dialErr := origDial(ctx, network, net.JoinHostPort(ip.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				lastErr = dialErr
			}
			if lastErr == nil {
				lastErr = fmt.Errorf("webhook 主机 %s 没有可用地址", h)
			}
			return nil, lastErr
		}
		transport = cloned
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	const maxResponse = 64 << 10
	response, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return fmt.Errorf("读取 Webhook 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := response
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	if len(response) > maxResponse {
		return fmt.Errorf("Webhook 响应过大,无法确认发送结果")
	}
	var result struct {
		Code          *json.Number `json:"code"`
		ErrCode       *json.Number `json:"errcode"`
		StatusCode    *json.Number `json:"StatusCode"`
		Msg           string       `json:"msg"`
		ErrMsg        string       `json:"errmsg"`
		StatusMessage string       `json:"StatusMessage"`
		Message       string       `json:"message"`
	}
	if err := json.Unmarshal(response, &result); err != nil {
		if feishu || dingtalk {
			return fmt.Errorf("Webhook 返回无效的业务响应: %w", err)
		}
		return nil // 自建服务允许空响应或纯文本。
	}
	if (feishu && result.Code == nil && result.StatusCode == nil) || (dingtalk && result.ErrCode == nil) {
		return fmt.Errorf("Webhook 响应缺少业务状态码,无法确认发送结果")
	}
	for _, status := range []struct {
		name    string
		code    *json.Number
		message string
	}{
		{"code", result.Code, result.Msg}, {"errcode", result.ErrCode, result.ErrMsg}, {"StatusCode", result.StatusCode, result.StatusMessage},
	} {
		if status.code == nil {
			continue
		}
		code, err := status.code.Int64()
		if err != nil {
			return fmt.Errorf("Webhook 返回无效的 %s: %s", status.name, status.code.String())
		}
		// Bark 和部分自建服务用 code=200 表示成功;飞书、钉钉用 0。
		if code != 0 && !(status.name == "code" && code == 200 && !feishu && !dingtalk) {
			detail := status.message
			if detail == "" {
				detail = result.Message
			}
			return fmt.Errorf("Webhook 返回错误 (%s %d): %s", status.name, code, detail)
		}
	}
	return nil
}

// Status 逐个通道体检。verifyRemote=true 时会真的去调远端(Telegram getMe / getChat),
// 只想知道"配没配"就传 false —— 监控循环里每轮都调远端会白白打 API。
func Status(state *app.State, verifyRemote bool) []Channel {
	cfg := state.Config.Get()
	out := make([]Channel, 0, 2)

	tg := Channel{Name: "Telegram"}
	tg.Configured = strings.TrimSpace(cfg.TgToken) != "" && strings.TrimSpace(cfg.TgChatID) != ""
	if tg.Configured {
		if verifyRemote {
			ok, reason := telegram.VerifyConfig(state)
			tg.OK, tg.Detail = ok, reason
		} else {
			tg.OK = true
		}
	} else {
		tg.Detail = "未配置 Bot Token 或 Chat ID"
	}
	out = append(out, tg)

	wh := Channel{Name: "Webhook"}
	url := strings.TrimSpace(cfg.NotifyWebhookURL)
	wh.Configured = url != ""
	if wh.Configured {
		if verifyRemote {
			if err := sendWebhook(url, "OVH 控制台:通知通道连通性测试"); err != nil {
				wh.Detail = err.Error()
			} else {
				wh.OK = true
			}
		} else {
			wh.OK = true
		}
	} else {
		wh.Detail = "未配置"
	}
	out = append(out, wh)

	return out
}

// AnyAvailable 是否至少还有一条通道可用。
// 监控据此决定要不要自停 —— 只要还有人能收到消息,监控就该继续跑。
func AnyAvailable(state *app.State, verifyRemote bool) (bool, string) {
	chans := Status(state, verifyRemote)
	reasons := make([]string, 0, len(chans))
	for _, c := range chans {
		if c.Configured && c.OK {
			return true, ""
		}
		if c.Configured {
			reasons = append(reasons, c.Name+": "+c.Detail)
		}
	}
	if len(reasons) == 0 {
		return false, "没有配置任何通知通道(Telegram / Webhook)"
	}
	return false, strings.Join(reasons, "; ")
}
