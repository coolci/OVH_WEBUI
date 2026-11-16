package handlers

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/repository"
)

// App 配对接口(网页端管理 + App 端兑换)。
//
// 安全防线（BUG-SEC-05）：
//   1. 单 IP 原子预占限流（reserve + release 共享同一把互斥锁，杜绝并发 TOCTOU 穿透 5 次上限）；
//   2. 入口前置严格校验 8 位配对码字符集（非法格式不打 SQLite、不消耗全局配额）；
//   3. 按 /24(/48) 网段隔离 + 提升全局防爆破上限至 600/min（8 位 32 进制 2^40 空间下仍不可爆破，同时彻底杜绝单攻击者 30 个垃圾包锁死全站合法配对）；
//   4. 全字段控制字符清洗，防止换行日志注入（CRLF Log Injection）。

var (
	pairFailureLimiter = &ipFailureWindow{max: 5, window: time.Minute, lockMs: 5 * time.Minute}
	pairSubnetLimiter  = &ipFailureWindow{max: 20, window: time.Minute, lockMs: time.Minute}
	pairGlobalLimiter  = &globalFailureWindow{max: 600, window: time.Minute, lockMs: time.Minute}
)

// ResetPairLimiters 清空配对限流器状态（仅供测试隔离使用）
func ResetPairLimiters() {
	pairFailureLimiter.reset()
	pairSubnetLimiter.reset()
	pairGlobalLimiter.reset()
}

const pairingCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func isValidPairingCodeFormat(code string) bool {
	if len(code) != 8 {
		return false
	}
	for i := 0; i < len(code); i++ {
		if strings.IndexByte(pairingCodeAlphabet, code[i]) < 0 {
			return false
		}
	}
	return true
}

func sanitizePairLogField(s string, maxRunes int) string {
	var b strings.Builder
	count := 0
	for _, r := range s {
		if r == '\r' || r == '\n' || r == '\t' {
			r = ' '
		} else if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
		count++
		if maxRunes > 0 && count >= maxRunes {
			break
		}
	}
	return strings.TrimSpace(b.String())
}

func pairSubnetKey(ip string) string {
	addr, err := netip.ParseAddr(strings.TrimSpace(ip))
	if err != nil {
		return "raw:" + ip
	}
	addr = addr.Unmap()
	if addr.Is4() {
		if p, perr := addr.Prefix(24); perr == nil {
			return p.String()
		}
	} else if addr.Is6() {
		if p, perr := addr.Prefix(48); perr == nil {
			return p.String()
		}
	}
	return addr.String()
}

// globalFailureWindow 全局(跨来源)原子失败预占 + 锁定
type globalFailureWindow struct {
	max      int
	window   time.Duration
	lockMs   time.Duration
	mu       sync.Mutex
	fails    []time.Time
	inflight int
	blocked  time.Time
}

func (w *globalFailureWindow) reset() {
	w.mu.Lock()
	w.fails = nil
	w.inflight = 0
	w.blocked = time.Time{}
	w.mu.Unlock()
}

func (w *globalFailureWindow) reserve() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	if !w.blocked.IsZero() && now.Sub(w.blocked) < w.lockMs {
		return false
	}
	keep := w.fails[:0]
	for _, t := range w.fails {
		if now.Sub(t) < w.window {
			keep = append(keep, t)
		}
	}
	w.fails = keep
	if len(w.fails)+w.inflight >= w.max {
		return false
	}
	w.inflight++
	return true
}

func (w *globalFailureWindow) release(succeeded bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.inflight > 0 {
		w.inflight--
	}
	if succeeded {
		return
	}
	now := time.Now()
	w.fails = append(w.fails, now)
	if len(w.fails) >= w.max {
		w.blocked = now
		w.fails = w.fails[:0]
	}
}

const maxTrackedPairIPs = 4096

// ipFailureWindow 原子预占式按 IP/网段失败计数（消除 allow + fail 分离的 TOCTOU 并发穿透）
type ipFailureWindow struct {
	max      int
	window   time.Duration
	lockMs   time.Duration
	mu       sync.Mutex
	fails    map[string][]time.Time
	inflight map[string]int
	blocked  map[string]time.Time
}

func (w *ipFailureWindow) reset() {
	w.mu.Lock()
	w.fails = nil
	w.inflight = nil
	w.blocked = nil
	w.mu.Unlock()
}

func (w *ipFailureWindow) ensureMapsLocked() {
	if w.fails == nil {
		w.fails = map[string][]time.Time{}
		w.inflight = map[string]int{}
		w.blocked = map[string]time.Time{}
	}
}

func (w *ipFailureWindow) evictIfNeededLocked(now time.Time) {
	if len(w.fails)+len(w.blocked) < maxTrackedPairIPs {
		return
	}
	for k, until := range w.blocked {
		if now.Sub(until) >= w.lockMs {
			delete(w.blocked, k)
		}
	}
	for k, ts := range w.fails {
		keep := ts[:0]
		for _, t := range ts {
			if now.Sub(t) < w.window {
				keep = append(keep, t)
			}
		}
		if len(keep) == 0 {
			delete(w.fails, k)
		} else {
			w.fails[k] = keep
		}
	}
	if len(w.fails)+len(w.blocked) >= maxTrackedPairIPs {
		for k := range w.fails {
			if w.inflight[k] > 0 {
				continue
			}
			delete(w.fails, k)
			if len(w.fails)+len(w.blocked) < maxTrackedPairIPs {
				break
			}
		}
	}
}

// reserve 在同一把互斥锁内原子检查失败数+在途数并占用在途配额，根绝并发穿透。
func (w *ipFailureWindow) reserve(key string) bool {
	if key == "" {
		key = "unknown"
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ensureMapsLocked()

	now := time.Now()
	if until, ok := w.blocked[key]; ok {
		if now.Sub(until) < w.lockMs {
			return false
		}
		delete(w.blocked, key)
	}
	if ts, ok := w.fails[key]; ok {
		keep := ts[:0]
		for _, t := range ts {
			if now.Sub(t) < w.window {
				keep = append(keep, t)
			}
		}
		if len(keep) == 0 {
			delete(w.fails, key)
		} else {
			w.fails[key] = keep
		}
	}
	if len(w.fails[key])+w.inflight[key] >= w.max {
		return false
	}
	w.evictIfNeededLocked(now)
	w.inflight[key]++
	return true
}

// release 归还在途配额并原子记录成功/失败结果。
func (w *ipFailureWindow) release(key string, succeeded bool) {
	if key == "" {
		key = "unknown"
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ensureMapsLocked()

	if cur := w.inflight[key]; cur <= 1 {
		delete(w.inflight, key)
	} else {
		w.inflight[key] = cur - 1
	}

	if succeeded {
		delete(w.fails, key)
		delete(w.blocked, key)
		return
	}

	now := time.Now()
	w.evictIfNeededLocked(now)
	w.fails[key] = append(w.fails[key], now)
	if len(w.fails[key]) >= w.max {
		w.blocked[key] = now
		delete(w.fails, key)
	}
}

// CreatePairingCode POST /api/app/pairing-codes —— 网页端生成二维码内容
func CreatePairingCode(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		code, expiresAt, err := state.DeviceRepo.CreatePairingCode()
		if err != nil {
			state.Logger.Error("生成配对码失败: "+sanitizePairLogField(err.Error(), 200), "app")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "生成配对码失败", "code": "E57703499"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"code":      code,
			"expiresAt": expiresAt,
			"pairUrl":   pairURL(c, code),
		})
	}
}

func isTrustedProxyPeer(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}

// pairURL 生成 App 可识别的配对 URL
func pairURL(c *gin.Context, code string) string {
	if pub := strings.TrimRight(strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")), "/"); pub != "" {
		return pub + "/api/app/pair#" + code
	}
	scheme := "https"
	if c.Request.TLS == nil {
		proto := strings.ToLower(strings.TrimSpace(c.Request.Header.Get("X-Forwarded-Proto")))
		if proto == "https" && isTrustedProxyPeer(c.Request.RemoteAddr) {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := c.Request.Host
	return scheme + "://" + host + "/api/app/pair#" + code
}

// RedeemPairingCode POST /api/app/pair —— App 凭码换令牌(白名单,免密钥)
func RedeemPairingCode(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		safeIP := sanitizePairLogField(ip, 64)

		// 1. 单 IP 原子预占：并发 20 个请求最多只有 max(5) 个能进入后续流程
		if !pairFailureLimiter.reserve(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "尝试次数过多,请 5 分钟后再试(配对码 2 分钟就会过期,先去网页重新生成)", "code": "EC57B85C6",
			})
			return
		}

		var body struct {
			Code       string `json:"code"`
			DeviceName string `json:"deviceName"`
		}
		_ = c.ShouldBindJSON(&body)
		code := strings.ToUpper(strings.TrimSpace(body.Code))
		if code == "" {
			pairFailureLimiter.release(ip, false)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少配对码", "code": "E6641AC88"})
			return
		}
		// 2. 严格校验 8 位配对码格式：非法输入不进数据库、不写原始脏串入日志、不消耗全局配额
		if !isValidPairingCodeFormat(code) {
			pairFailureLimiter.release(ip, false)
			state.Logger.Warn("App 配对格式非法 ip="+safeIP, "app")
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "配对码格式非法 —— 应为 8 位大写字母或数字",
				"code":    "E79820B5A",
			})
			return
		}

		// 3. 网段隔离闸 + 全局防分布式撞库闸
		subnet := pairSubnetKey(ip)
		if !pairSubnetLimiter.reserve(subnet) {
			pairFailureLimiter.release(ip, false)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "当前网段配对尝试过于频繁,请 1 分钟后再试", "code": "E93CC6F2D",
			})
			return
		}
		if !pairGlobalLimiter.reserve() {
			pairSubnetLimiter.release(subnet, false)
			pairFailureLimiter.release(ip, false)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "配对尝试过于频繁,接口已暂时锁定 1 分钟 —— 稍后再试或联系管理员", "code": "E93CC6F2D",
			})
			return
		}

		name := sanitizePairLogField(body.DeviceName, 60)
		token, deviceID, err := state.DeviceRepo.RedeemPairingCode(code, name)
		if err != nil {
			pairGlobalLimiter.release(false)
			pairSubnetLimiter.release(subnet, false)
			pairFailureLimiter.release(ip, false)

			msg, code2 := "配对失败", "ED03B15C6"
			switch {
			case errors.Is(err, repository.ErrPairingCodeNotFound):
				msg, code2 = "配对码不存在 —— 检查大小写,或去网页重新生成", "E79820B5A"
			case errors.Is(err, repository.ErrPairingCodeUsed):
				msg, code2 = "配对码已被使用(一码一机)。去网页重新生成再试", "EA92792C8"
			case errors.Is(err, repository.ErrPairingCodeExpired):
				msg, code2 = "配对码已过期(有效期 2 分钟)。去网页重新生成再试", "E21D475B2"
			}
			state.Logger.Warn("App 配对失败 ip="+safeIP+" code="+code+" 原因="+sanitizePairLogField(err.Error(), 120), "app")
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": msg, "code": code2})
			return
		}

		pairGlobalLimiter.release(true)
		pairSubnetLimiter.release(subnet, true)
		pairFailureLimiter.release(ip, true)

		state.Logger.Info("App 配对成功 device=#"+strconv.FormatInt(deviceID, 10)+" name="+name, "app")
		c.JSON(http.StatusOK, gin.H{
			"success":       true,
			"token":         token,
			"deviceId":      deviceID,
			"serverVersion": Version,
		})
	}
}

// ListAppDevices GET /api/app/devices —— 网页端设备管理列表
func ListAppDevices(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		devs, err := state.DeviceRepo.ListDevices()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		out := make([]gin.H, 0, len(devs))
		for _, d := range devs {
			row := gin.H{"id": d.ID, "name": d.Name, "createdAt": d.CreatedAt}
			if d.LastUsed.Valid {
				row["lastUsedAt"] = d.LastUsed.Time
			}
			row["revoked"] = d.Revoked.Valid
			if d.Revoked.Valid {
				row["revokedAt"] = d.Revoked.Time
			}
			out = append(out, row)
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "devices": out, "total": len(out)})
	}
}

// RevokeAppDevice DELETE /api/app/devices/:id —— 吊销(幂等)
func RevokeAppDevice(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "设备 id 非法", "code": "E1703A9CF"})
			return
		}
		if err := state.DeviceRepo.RevokeDevice(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("App 设备已吊销 #"+strconv.FormatInt(id, 10), "app")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "设备已吊销,该设备的令牌立即失效", "code": "EB183A580"})
	}
}
