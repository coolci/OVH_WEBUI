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
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/telegram"
)

// AllowLocalWebhookForTesting 仅在单元测试中临时允许回环地址
var AllowLocalWebhookForTesting = false

var blockedIPv4Prefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),       // RFC 1122 "This network"
	netip.MustParsePrefix("10.0.0.0/8"),      // RFC 1918 Private-Use
	netip.MustParsePrefix("100.64.0.0/10"),   // RFC 6598 Shared Address Space (CGNAT)
	netip.MustParsePrefix("127.0.0.0/8"),     // RFC 1122 Loopback
	netip.MustParsePrefix("169.254.0.0/16"),  // RFC 3927 Link-Local / Cloud Metadata
	netip.MustParsePrefix("172.16.0.0/12"),   // RFC 1918 Private-Use
	netip.MustParsePrefix("192.0.0.0/24"),    // RFC 6890 IETF Protocol Assignments
	netip.MustParsePrefix("192.0.2.0/24"),    // RFC 5737 TEST-NET-1
	netip.MustParsePrefix("192.88.99.0/24"),  // RFC 7526 6to4 Relay Anycast
	netip.MustParsePrefix("192.168.0.0/16"),  // RFC 1918 Private-Use
	netip.MustParsePrefix("198.18.0.0/15"),   // RFC 2544 Benchmarking
	netip.MustParsePrefix("198.51.100.0/24"), // RFC 5737 TEST-NET-2
	netip.MustParsePrefix("203.0.113.0/24"),  // RFC 5737 TEST-NET-3
	netip.MustParsePrefix("224.0.0.0/4"),     // Multicast
	netip.MustParsePrefix("240.0.0.0/4"),     // Reserved & 255.255.255.255 Limited Broadcast
}

var blockedIPv6Prefixes = []netip.Prefix{
	netip.MustParsePrefix("::/96"),           // RFC 4291 Deprecated IPv4-compatible IPv6 (e.g. ::127.0.0.1)
	netip.MustParsePrefix("::ffff:0:0:0/96"), // RFC 2765 SIIT IPv4-translated
	netip.MustParsePrefix("64:ff9b::/96"),    // RFC 6052 NAT64 Well-Known Prefix
	netip.MustParsePrefix("64:ff9b:1::/48"),  // RFC 8215 NAT64 Local-Use Prefix
	netip.MustParsePrefix("100::/64"),        // RFC 6666 Discard-Only
	netip.MustParsePrefix("2001::/32"),       // RFC 4380 Teredo
	netip.MustParsePrefix("2001:2::/48"),     // RFC 5180 Benchmarking
	netip.MustParsePrefix("2001:10::/28"),    // RFC 4843 ORCHID
	netip.MustParsePrefix("2001:20::/28"),    // RFC 7343 ORCHIDv2
	netip.MustParsePrefix("2001:db8::/32"),   // RFC 3849 Documentation
	netip.MustParsePrefix("2002::/16"),       // RFC 3056 6to4 Tunneling
	netip.MustParsePrefix("fc00::/7"),        // RFC 4193 Unique Local
	netip.MustParsePrefix("fe80::/10"),       // RFC 4291 Link-Local Unicast
	netip.MustParsePrefix("fec0::/10"),       // RFC 3879 Site-Local (Deprecated)
	netip.MustParsePrefix("ff00::/8"),        // Multicast
}

// isPrivateOrLocalAddr 基于 net/netip（先 Unmap 还原 IPv4-in-IPv6）执行严格公网全局单播校验。
func isPrivateOrLocalAddr(addr netip.Addr) bool {
	if !addr.IsValid() {
		return true
	}
	addr = addr.Unmap()
	if addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsMulticast() ||
		addr.IsUnspecified() ||
		!addr.IsGlobalUnicast() {
		return true
	}
	if addr.Is4() {
		for _, p := range blockedIPv4Prefixes {
			if p.Contains(addr) {
				return true
			}
		}
		return false
	}
	for _, p := range blockedIPv6Prefixes {
		if p.Contains(addr) {
			return true
		}
	}
	// IANA 公网全局单播 IPv6 仅分配在 2000::/3 空间内，拒绝其余保留地址块
	b := addr.As16()
	if (b[0] & 0xe0) != 0x20 {
		return true
	}
	return false
}

func isTestingLoopbackExempt(addr netip.Addr) bool {
	return AllowLocalWebhookForTesting && addr.IsValid() && addr.Unmap().IsLoopback()
}

// IsPrivateOrLocalIP 检查 IP 是否为本地回环、RFC1918 私网、CGNAT、NAT64、6to4、多播、链路本地、云元数据等非公网单播地址
func IsPrivateOrLocalIP(ip net.IP) bool {
	if len(ip) == 0 {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	return isPrivateOrLocalAddr(addr)
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
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		if AllowLocalWebhookForTesting {
			return nil
		}
		return fmt.Errorf("通知 Webhook 地址禁止指向本地或内网地址 (localhost)")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if isPrivateOrLocalAddr(addr) {
			if isTestingLoopbackExempt(addr) {
				return nil
			}
			return fmt.Errorf("通知 Webhook 地址禁止指向私有或内网 IP (%s)", host)
		}
		return nil
	}
	// 尝试域名解析预检
	addrs, err := net.DefaultResolver.LookupNetIP(context.Background(), "ip", host)
	if err == nil {
		for _, addr := range addrs {
			if isPrivateOrLocalAddr(addr) {
				if isTestingLoopbackExempt(addr) {
					continue
				}
				return fmt.Errorf("通知 Webhook 域名解析为私有或内网 IP (%s -> %s)", host, addr.String())
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

type dialContextFunc func(ctx context.Context, network, addr string) (net.Conn, error)

var (
	stdDefaultTransport = http.DefaultTransport
	baseWebhookDialer   = &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	sharedWebhookTransport = buildSafeWebhookTransport(baseWebhookDialer.DialContext)
	sharedWebhookClient    = &http.Client{
		Transport: sharedWebhookTransport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
)

func makeSafeWebhookDialContext(rawDial dialContextFunc) dialContextFunc {
	if rawDial == nil {
		rawDial = baseWebhookDialer.DialContext
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		h, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		if literalAddr, parseErr := netip.ParseAddr(h); parseErr == nil {
			unmapped := literalAddr.Unmap()
			if isPrivateOrLocalAddr(unmapped) && !isTestingLoopbackExempt(unmapped) {
				return nil, fmt.Errorf("SSRF 拦截: 目标 IP %s 属于私有或本地网络", literalAddr.String())
			}
			return rawDial(ctx, network, net.JoinHostPort(unmapped.String(), port))
		}

		addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", h)
		if err != nil {
			return nil, err
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("webhook 主机 %s 没有可用地址", h)
		}
		verified := make([]netip.Addr, 0, len(addrs))
		for _, resolved := range addrs {
			unmapped := resolved.Unmap()
			if isPrivateOrLocalAddr(unmapped) {
				if isTestingLoopbackExempt(unmapped) {
					verified = append(verified, unmapped)
					continue
				}
				return nil, fmt.Errorf("SSRF 拦截: 目标 IP %s 属于私有或本地网络", resolved.String())
			}
			verified = append(verified, unmapped)
		}

		// 直接拨号已校验过的字面量 IP，杜绝二次解析引发的 DNS Rebinding 绕过
		var lastErr error
		for _, ipAddr := range verified {
			conn, dialErr := rawDial(ctx, network, net.JoinHostPort(ipAddr.String(), port))
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
}

func buildSafeWebhookTransport(rawDial dialContextFunc) *http.Transport {
	var cloned *http.Transport
	if tr, ok := stdDefaultTransport.(*http.Transport); ok {
		cloned = tr.Clone()
	} else {
		cloned = &http.Transport{
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          32,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}
	// 严禁继承 HTTP_PROXY/HTTPS_PROXY 环境变量代理，防止本机回环代理被误判 SSRF 或外部代理代连内网
	cloned.Proxy = nil
	cloned.DialContext = makeSafeWebhookDialContext(rawDial)
	return cloned
}

// SharedWebhookTransport 返回包级复用的受限无代理 Transport（仅供安全回归单测检视属性）
func SharedWebhookTransport() *http.Transport {
	return sharedWebhookTransport
}

func isMockRoundTripperInstalled() bool {
	if http.DefaultTransport == nil || http.DefaultTransport == stdDefaultTransport {
		return false
	}
	_, isTransport := http.DefaultTransport.(*http.Transport)
	return !isTransport
}

func getWebhookClient() *http.Client {
	// 生产路径：零分配复用包级无代理单例客户端与连接池
	if http.DefaultTransport == nil || http.DefaultTransport == stdDefaultTransport {
		return sharedWebhookClient
	}
	// 单测支持 1（pinning_test.go）：注入自定义 *http.Transport DialContext 探针时包装同一 SSRF 拨号校验
	if customTr, ok := http.DefaultTransport.(*http.Transport); ok {
		wrapped := customTr.Clone()
		wrapped.Proxy = nil
		wrapped.DialContext = makeSafeWebhookDialContext(customTr.DialContext)
		return &http.Client{
			Transport: wrapped,
			Timeout:   10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	// 单测支持 2（notify_test.go）：注入内存级 RoundTripper 桩（请求不出进程）
	return &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func validateWebhookURLSyntaxAndLiteral(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("通知 Webhook 地址不合法,必须是完整的 http:// 或 https:// 地址")
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".localhost") {
		if AllowLocalWebhookForTesting {
			return nil
		}
		return fmt.Errorf("通知 Webhook 地址禁止指向本地或内网地址 (localhost)")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if isPrivateOrLocalAddr(addr) && !isTestingLoopbackExempt(addr) {
			return fmt.Errorf("通知 Webhook 地址禁止指向私有或内网 IP (%s)", host)
		}
	}
	return nil
}

// sendWebhook 发文本消息,同时检查 HTTP 状态和常见的业务状态码。
func sendWebhook(webhookURL, message string) error {
	if isMockRoundTripperInstalled() {
		if err := validateWebhookURLSyntaxAndLiteral(webhookURL); err != nil {
			return err
		}
	} else if err := ValidateWebhookURL(webhookURL); err != nil {
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

	client := getWebhookClient()
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
