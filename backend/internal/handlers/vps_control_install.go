package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/ovh"
)

// templatesCache 缓存模板列表,key = accountID:serviceName
var (
	templatesCacheMu sync.Mutex
	templatesCache   = map[string]templatesCacheEntry{}
)

const templatesCacheTTL = 10 * time.Minute

type templatesCacheEntry struct {
	list    []gin.H
	kind    string
	expires time.Time
}

// GetVpsCurrentOS GET /api/vps-control/:service_name/current-os
//
// 只走 /vps/{name}/images/current(三区都有,BETA)。
// 旧路 /vps/{name}/distribution 被 OVH 标记废弃、2026-10-15 删除 ——
// 按约定废弃端点不再调用。
func GetVpsCurrentOS(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		var img map[string]interface{}
		if err := client.Get("/vps/"+svc+"/images/current", &img); err != nil {
			if ovhIsGone(err) {
				c.JSON(http.StatusOK, gin.H{"success": true, "currentOS": nil, "removed": true})
				return
			}
			state.Logger.Error("VPS "+svc+" 读取当前系统失败: "+err.Error(), "vps_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		name, _ := img["name"].(string)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"currentOS": gin.H{
				"id":           img["id"],
				"name":         name,
				"distribution": inferDistributionFromName(name),
				"bitFormat":    64,
				"locale":       "",
				"source":       "images/current",
			},
		})
	}
}

// GetVpsTemplates GET /api/vps-control/:service_name/templates
//
// 只走 /vps/{name}/images/available(string[] imageId,三区都有,BETA)。
// 旧路 /vps/{name}/templates(EU/CA,long[] templateId)被 OVH 标记废弃、
// 2026-10-15 删除 —— 按约定废弃端点不再调用,"先试旧的再退"都不留。
// 统一返回 { id, name, distribution, bitFormat, locale, availableLanguage, kind },
// kind 恒为 "imageId",字段保留是为了前端兼容旧响应结构。
//
// 缓存:同一账户的同一 VPS 模板列表缓存 10 分钟。详情拉取走 10 并发。
func GetVpsTemplates(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		acc, _ := ovhAccountFor(state, c)
		cacheKey := acc.ID + ":" + svc

		// 命中缓存直接走
		templatesCacheMu.Lock()
		if entry, ok := templatesCache[cacheKey]; ok && time.Now().Before(entry.expires) {
			list, kind := entry.list, entry.kind
			templatesCacheMu.Unlock()
			c.JSON(http.StatusOK, gin.H{
				"success": true, "templates": list, "total": len(list), "kind": kind, "cached": true,
			})
			return
		}
		templatesCacheMu.Unlock()

		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		var imageIDs []string
		if err := client.Get("/vps/"+svc+"/images/available", &imageIDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		list, failed := buildUsImageList(client, svc, imageIDs)
		respondTemplates(state, c, cacheKey, svc, list, failed, len(imageIDs), "imageId")
	}
}

// respondTemplates 统一处理「详情拉取部分/全部失败」的情形。
//
// 30+ 个模板用 10 并发打 OVH,一次瞬时 429 就可能让详情全挂。以前失败位被静默跳过,
// 空列表照样按 10 分钟 TTL 写进缓存,前端显示「账户没有模板」且刷新无效 —— 必须区分开:
//   - 全挂(拿到了 id 却一条详情都没有)返 500,让用户知道是读失败、可以立刻重试
//   - 部分挂:照常返回,但不写缓存,免得残缺列表被钉住 10 分钟
func respondTemplates(state *app.State, c *gin.Context, cacheKey, svc string, list []gin.H, failed, total int, kind string) {
	// images 分支详情失败时会用 id 兜底出一条记录,所以不能只看 len(list)==0,
	// 还要看失败数是不是把 total 吃满了 —— 那种「一列裸 id」的列表同样是读失败,不是真实模板表。
	if total > 0 && (len(list) == 0 || failed == total) {
		state.Logger.Error(fmt.Sprintf("VPS %s 模板详情全部拉取失败(%d 条)", svc, total), "vps_control")
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "读取系统模板详情失败(可能被 OVH 限流),请稍后重试",
		})
		return
	}
	if failed == 0 {
		cacheTemplates(cacheKey, list, kind)
	} else {
		state.Logger.Warn(fmt.Sprintf("VPS %s 模板详情部分失败(%d/%d),本次不写缓存", svc, failed, total), "vps_control")
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true, "templates": list, "total": len(list), "kind": kind,
		"partial": failed > 0, "failed": failed,
	})
}

// parallelGetVpsDetails 跟 util.go 的 parallelGetDetails 一样并发拉详情,区别是额外返回失败个数。
// 模板列表要靠这个计数区分「账户真的没有模板」和「详情被限流拉挂了」。
func parallelGetVpsDetails(client *ovhsdk.Client, paths []string, concurrency int) ([]map[string]interface{}, int) {
	if concurrency <= 0 {
		concurrency = 10
	}
	results := make([]map[string]interface{}, len(paths))
	failed := make([]bool, len(paths))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, path := range paths {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, p string) {
			defer wg.Done()
			defer func() { <-sem }()
			var d map[string]interface{}
			if err := client.Get(p, &d); err != nil {
				failed[idx] = true
				return
			}
			results[idx] = d
		}(i, path)
	}
	wg.Wait()
	n := 0
	for _, f := range failed {
		if f {
			n++
		}
	}
	return results, n
}

func cacheTemplates(key string, list []gin.H, kind string) {
	templatesCacheMu.Lock()
	templatesCache[key] = templatesCacheEntry{
		list:    list,
		kind:    kind,
		expires: time.Now().Add(templatesCacheTTL),
	}
	templatesCacheMu.Unlock()
}

// buildUsImageList /images/available 分支(三区共用,不只美区)。vps.Image 只有 { id, name },
// 从 name 推断 distribution。10 并发拉。
// 详情失败时用 id 兜底 name(列表仍可用),但失败个数要返回给调用方,失败就不写缓存。
func buildUsImageList(client *ovhsdk.Client, svc string, ids []string) ([]gin.H, int) {
	paths := make([]string, len(ids))
	for i, id := range ids {
		paths[i] = "/vps/" + svc + "/images/available/" + id
	}
	details, failed := parallelGetVpsDetails(client, paths, 10)
	list := []gin.H{}
	for i, id := range ids {
		d := details[i]
		nm := id
		if d != nil {
			if v, ok := d["name"].(string); ok && v != "" {
				nm = v
			}
		}
		dist := inferDistributionFromName(nm)
		list = append(list, gin.H{
			"id":                id,
			"name":              nm,
			"distribution":      dist,
			"bitFormat":         64,
			"locale":            "",
			"availableLanguage": []string{},
		})
	}
	return sortTemplatesByDistribution(list), failed
}

// inferDistributionFromName 从 image name 推 distribution(US Image 没单独字段)
func inferDistributionFromName(name string) string {
	lc := strings.ToLower(name)
	for _, d := range []string{"debian", "ubuntu", "centos", "rocky", "almalinux", "fedora", "windows", "freebsd", "arch"} {
		if strings.Contains(lc, d) {
			return d
		}
	}
	return ""
}

// sortTemplatesByDistribution 把 debian / ubuntu / centos 等常见 distro 排前面
func sortTemplatesByDistribution(list []gin.H) []gin.H {
	priority := []string{"debian", "ubuntu", "centos", "rocky", "almalinux", "windows"}
	getPriority := func(t gin.H) int {
		d := strings.ToLower(fmt.Sprintf("%v", t["distribution"]))
		for i, p := range priority {
			if strings.Contains(d, p) {
				return i
			}
		}
		return len(priority)
	}
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && (getPriority(list[j-1]) > getPriority(list[j]) ||
			(getPriority(list[j-1]) == getPriority(list[j]) &&
				fmt.Sprintf("%v", list[j-1]["name"]) > fmt.Sprintf("%v", list[j]["name"]))); j-- {
			list[j-1], list[j] = list[j], list[j-1]
		}
	}
	return list
}

// ReinstallVps POST /api/vps-control/:service_name/reinstall
//
// 只走 /vps/{name}/rebuild(三区都有,BETA),body vps.rebuild.post:
// { imageId: string(必填), sshKey: string(单个 key 名), installRTM, doNotSendPassword, ... }
// 旧路 /vps/{name}/reinstall(EU/CA,long templateId)被 OVH 标记废弃、2026-10-15
// 删除 —— 按约定废弃端点不再调用。模板列表现在恒为镜像制(见 GetVpsTemplates),
// templateId 一律按 imageId(string)处理;数字值(旧前端缓存)转成字符串发出去,
// OVH 不认会在 400 里说清楚。
func ReinstallVps(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		acc, _ := ovhAccountFor(state, c)

		var body struct {
			TemplateID        interface{} `json:"templateId"` // 兼容旧字段名;语义是 imageId(string),数字会被转成字符串
			SSHKey            []string    `json:"sshKey"`
			DoNotSendPassword bool        `json:"doNotSendPassword"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.TemplateID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 templateId"})
			return
		}

		// imageId 必须是 string;数字(旧缓存)转字符串
		imageID, ok := body.TemplateID.(string)
		if !ok {
			if tid, isNum := numconv.ToInt64(body.TemplateID); isNum {
				imageID = strconv.FormatInt(tid, 10)
			} else {
				imageID = fmt.Sprintf("%v", body.TemplateID)
			}
		}
		if imageID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "templateId 不能为空"})
			return
		}
		// sshKey 在 vps.rebuild.post 里是单个 string(key 名),不是数组;language /
		// softwareId 不在该模型里,不能塞 —— OVH 对未知字段没有宽容度承诺。
		params := map[string]interface{}{
			"imageId":           imageID,
			"doNotSendPassword": body.DoNotSendPassword,
			"installRTM":        false,
		}
		if len(body.SSHKey) > 0 {
			params["sshKey"] = body.SSHKey[0]
		}
		var task map[string]interface{}
		if err := client.Post("/vps/"+svc+"/rebuild", params, &task); err != nil {
			state.Logger.Error("VPS "+svc+" rebuild 失败: "+err.Error(), "vps_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info(fmt.Sprintf("VPS %s rebuild 任务已创建: imageId=%s (endpoint=%s)", svc, imageID, acc.Endpoint), "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "重装任务已创建", "task": task})
	}
}

// GetVpsTasks GET /api/vps-control/:service_name/tasks
func GetVpsTasks(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var ids []int64
		if err := client.Get("/vps/"+svc+"/tasks", &ids); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		// schema 只承诺 long[],对顺序没有任何约定。以前直接取切片尾部 10 个当「最近」,
		// 但 OVH 的 id 并非单调递增(有时新任务 id 偏小),倒序取会把真正的最近任务漏掉。
		// 正确做法:全量 id 按数字降序排,前 10 个才是真正的最近 10 个。
		sorted := make([]int64, len(ids))
		copy(sorted, ids)
		for i := 1; i < len(sorted); i++ {
			for j := i; j > 0 && sorted[j] > sorted[j-1]; j-- {
				sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
			}
		}
		if len(sorted) > 10 {
			sorted = sorted[:10]
		}
		paths := make([]string, len(sorted))
		for i, id := range sorted {
			paths[i] = fmt.Sprintf("/vps/%s/tasks/%d", svc, id)
		}
		// 并发拉取最近 10 条任务详情
		details := parallelGetDetails(client, paths, 5)
		list := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "tasks": list, "total": len(ids)})
	}
}

// GetVpsTaskDetail GET /api/vps-control/:service_name/tasks/:task_id
func GetVpsTaskDetail(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		taskID := c.Param("task_id")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var d map[string]interface{}
		if err := client.Get(fmt.Sprintf("/vps/%s/tasks/%s", svc, taskID), &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "task": d})
	}
}
