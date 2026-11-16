package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/updater"
)

// Version 当前二进制版本。build.sh 用 -ldflags "-X github.com/ovh-webui/server/internal/handlers.Version=x.y.z" 注入。
// 默认 "dev" 给 go run / 未注入的 build。
var Version = "dev"

// GetVersion GET /api/version  无需鉴权,前端启动时拿来显示
func GetVersion(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"version": Version,
		})
	}
}

const (
	updateUserAgent      = "OVH-Console-UpdateChecker"
	updateCheckCacheTTL  = 15 * time.Minute
	maxReleaseJSONBytes  = 1 << 20 // 1 MiB 响应体上限（BUG-SEC-08）
)

type cachedUpdateResult struct {
	payload   gin.H
	expiresAt time.Time
}

var (
	updateCacheMu   sync.RWMutex
	updateCacheByKey = map[string]cachedUpdateResult{}
	updateCheckGroup singleflight.Group
	updateHTTPClient = &http.Client{Timeout: 15 * time.Second}
)

// ResetUpdateCheckCache 清空更新检查内存缓存（仅供单元测试隔离使用）
func ResetUpdateCheckCache() {
	updateCacheMu.Lock()
	updateCacheByKey = map[string]cachedUpdateResult{}
	updateCacheMu.Unlock()
}

// githubRelease 只挑要用的字段反序列化
type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	HTMLURL     string `json:"html_url"`
	PublishedAt string `json:"published_at"`
	Body        string `json:"body"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
}

// trimV 去掉 tag 的 v 前缀
func trimV(s string) string { return strings.TrimPrefix(strings.TrimSpace(s), "v") }

// parseSemver 把 "v0.0.2" / "0.1.10" 切成 [0 0 2] / [0 1 10]
func parseSemver(s string) [3]int {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.SplitN(s, ".", 3)
	var v [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(parts[i])
		v[i] = n
	}
	return v
}

// semverGreater latest > current ?
func semverGreater(latest, current string) bool {
	l := parseSemver(latest)
	c := parseSemver(current)
	for i := 0; i < 3; i++ {
		if l[i] > c[i] {
			return true
		}
		if l[i] < c[i] {
			return false
		}
	}
	return false
}

func cloneUpdatePayload(in gin.H) gin.H {
	out := make(gin.H, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// CheckUpdate GET /api/version/check-update（需鉴权，BUG-SEC-08）：
//   - 15 分钟内存缓存 + singleflight 并发请求合并，杜绝打满上游 GitHub API 匿名限额；
//   - io.LimitReader(resp.Body, 1<<20) 限制最大响应体；
//   - 上游非 200 状态统一收敛为 502 BadGateway，不反射上游状态码与原始响应体。
func CheckUpdate(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		releaseURL := updater.LatestReleaseURL()
		cacheKey := releaseURL + "|" + Version
		now := time.Now()

		updateCacheMu.RLock()
		if cached, ok := updateCacheByKey[cacheKey]; ok && now.Before(cached.expiresAt) {
			respCopy := cloneUpdatePayload(cached.payload)
			updateCacheMu.RUnlock()
			c.JSON(http.StatusOK, respCopy)
			return
		}
		updateCacheMu.RUnlock()

		val, err, _ := updateCheckGroup.Do(cacheKey, func() (interface{}, error) {
			updateCacheMu.RLock()
			if cached, ok := updateCacheByKey[cacheKey]; ok && time.Now().Before(cached.expiresAt) {
				cp := cloneUpdatePayload(cached.payload)
				updateCacheMu.RUnlock()
				return cp, nil
			}
			updateCacheMu.RUnlock()

			req, reqErr := http.NewRequest(http.MethodGet, releaseURL, nil)
			if reqErr != nil {
				return nil, fmt.Errorf("构建更新检查请求失败")
			}
			req.Header.Set("Accept", "application/vnd.github+json")
			req.Header.Set("User-Agent", updateUserAgent)

			resp, doErr := updateHTTPClient.Do(req)
			if doErr != nil {
				if state != nil && state.Logger != nil {
					state.Logger.Warn("update check 拉取失败: "+doErr.Error(), "version")
				}
				return nil, fmt.Errorf("连接上游版本服务失败")
			}
			defer resp.Body.Close()

			limited := io.LimitReader(resp.Body, maxReleaseJSONBytes+1)
			body, readErr := io.ReadAll(limited)
			if readErr != nil {
				return nil, fmt.Errorf("读取上游版本响应失败")
			}
			if len(body) > maxReleaseJSONBytes {
				if state != nil && state.Logger != nil {
					state.Logger.Warn("update check 响应体超过 1MB 上限，已拦截", "version")
				}
				return nil, fmt.Errorf("上游版本响应体超过安全上限")
			}
			if resp.StatusCode != http.StatusOK {
				if state != nil && state.Logger != nil {
					snippet := string(body)
					if len(snippet) > 200 {
						snippet = snippet[:200]
					}
					state.Logger.Warn(fmt.Sprintf("update check 上游返回非 200 (%d): %s", resp.StatusCode, strings.TrimSpace(snippet)), "version")
				}
				return nil, fmt.Errorf("上游版本服务暂不可用")
			}

			var rel githubRelease
			if decErr := json.Unmarshal(body, &rel); decErr != nil {
				return nil, fmt.Errorf("解析上游版本信息失败")
			}

			latest := strings.TrimPrefix(rel.TagName, "v")
			hasUpdate := Version != "dev" && semverGreater(latest, Version)
			payload := gin.H{
				"current":     Version,
				"latest":      latest,
				"tag":         rel.TagName,
				"name":        rel.Name,
				"hasUpdate":   hasUpdate,
				"url":         rel.HTMLURL,
				"publishedAt": rel.PublishedAt,
				"body":        rel.Body,
				"prerelease":  rel.Prerelease,
				"checkedAt":   time.Now().UTC().Format(time.RFC3339),
				"inContainer": updater.InContainer(),
				"updateHint":  containerHintIfNeeded(),
			}

			updateCacheMu.Lock()
			updateCacheByKey[cacheKey] = cachedUpdateResult{
				payload:   cloneUpdatePayload(payload),
				expiresAt: time.Now().Add(updateCheckCacheTTL),
			}
			updateCacheMu.Unlock()
			return payload, nil
		})

		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   err.Error(),
				"code":    "UPDATE_CHECK_FAILED",
				"current": Version,
			})
			return
		}

		c.JSON(http.StatusOK, val.(gin.H))
	}
}

// containerHintIfNeeded 容器里返回更新指引,否则空串。
func containerHintIfNeeded() string {
	if updater.InContainer() {
		return updater.ContainerUpdateHint
	}
	return ""
}
