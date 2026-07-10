package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
)

// 模板列表缓存(账户 + 服务名 维度)。OVH 模板表基本不变,缓存 10 分钟避免每次开重装对话框都拉一遍。
// 实测一台 VPS 30+ 模板,每个还要查详情,首次冷加载 5-15 秒;缓存命中后立即返回。
type templatesCacheEntry struct {
	list    []gin.H
	kind    string
	expires time.Time
}

var (
	templatesCacheMu sync.Mutex
	templatesCache   = map[string]templatesCacheEntry{}
)

const templatesCacheTTL = 10 * time.Minute

// GetVpsCurrentOS GET /api/vps-control/:service_name/current-os
//
// 当前安装的系统信息。两个端点:
//   /vps/{name}/distribution     - EU PRODUCTION,返完整 vps.Template (id, name, distribution, bitFormat, locale)
//   /vps/{name}/images/current   - EU/US BETA,返简化 vps.Image (id, name)
// EU 优先用前者(信息全),失败/US 退后者,前端按 name 推 distribution。
func GetVpsCurrentOS(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}

		// 先试 EU /distribution(完整)
		var tpl map[string]interface{}
		if err := client.Get("/vps/"+svc+"/distribution", &tpl); err == nil && tpl != nil {
			name, _ := tpl["name"].(string)
			dist, _ := tpl["distribution"].(string)
			bf := 64
			if v, ok := numconv.ToInt64(tpl["bitFormat"]); ok {
				bf = int(v)
			}
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"currentOS": gin.H{
					"id":           tpl["id"],
					"name":         name,
					"distribution": dist,
					"bitFormat":    bf,
					"locale":       valueOr(tpl, "locale", ""),
					"source":       "distribution",
				},
			})
			return
		}

		// US 退路 /images/current(简化)
		var img map[string]interface{}
		if err := client.Get("/vps/"+svc+"/images/current", &img); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": true, "currentOS": nil})
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
// EU 走 /vps/{name}/templates (long[] templateId);
// US 没这个端点,走 /vps/{name}/images/available (string[] imageId)。
// 统一封装返回 { id, name, distribution, bitFormat, locale, availableLanguage, kind }
// kind ∈ { "templateId", "imageId" },前端按此决定 reinstall body 用哪个字段。
//
// 缓存:同一账户的同一 VPS 模板列表缓存 10 分钟。详情拉取走 10 并发。
func GetVpsTemplates(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		acc, _ := ovhAccountFor(state, c)
		cacheKey := acc.ID + ":" + svc

		// 命中缓存直接返
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

		// 先试 EU 路径
		var euIDs []int64
		if err := client.Get("/vps/"+svc+"/templates", &euIDs); err == nil && len(euIDs) > 0 {
			list := buildEuTemplateList(client, svc, euIDs)
			cacheTemplates(cacheKey, list, "templateId")
			c.JSON(http.StatusOK, gin.H{"success": true, "templates": list, "total": len(list), "kind": "templateId"})
			return
		}

		// US 退路
		var imageIDs []string
		if err := client.Get("/vps/"+svc+"/images/available", &imageIDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		list := buildUsImageList(client, svc, imageIDs)
		cacheTemplates(cacheKey, list, "imageId")
		c.JSON(http.StatusOK, gin.H{"success": true, "templates": list, "total": len(list), "kind": "imageId"})
	}
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

// buildEuTemplateList EU vps.Template 完整字段。10 并发拉详情,30+ 模板从 15s → 2s。
func buildEuTemplateList(client *ovhsdk.Client, svc string, ids []int64) []gin.H {
	keys := make([]interface{}, len(ids))
	for i, id := range ids {
		keys[i] = id
	}
	details := parallelGetDetails(client, keys, func(k interface{}) string {
		return fmt.Sprintf("/vps/%s/templates/%v", svc, k)
	}, 10)
	return assembleAndSortTemplates(ids, details, "templateId", svc, true)
}

// buildUsImageList US vps.Image 只有 { id, name },从 name 推断 distribution。10 并发。
func buildUsImageList(client *ovhsdk.Client, svc string, ids []string) []gin.H {
	details := parallelGetStringKeys(client, ids, func(id string) string {
		return "/vps/" + svc + "/images/available/" + id
	}, 10)
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
	return sortTemplatesByDistribution(list)
}

// assembleAndSortTemplates EU 路径专用:把 detail map 转成统一 shape 并排序
func assembleAndSortTemplates(ids []int64, details []map[string]interface{}, _kind string, _svc string, _isEU bool) []gin.H {
	list := []gin.H{}
	for i, id := range ids {
		d := details[i]
		if d == nil {
			continue
		}
		bf := 64
		if v, ok := numconv.ToInt64(d["bitFormat"]); ok {
			bf = int(v)
		}
		langs := []string{}
		if arr, ok := d["availableLanguage"].([]interface{}); ok {
			for _, l := range arr {
				if s, ok := l.(string); ok {
					langs = append(langs, s)
				}
			}
		}
		list = append(list, gin.H{
			"id":                id,
			"name":              valueOr(d, "name", ""),
			"distribution":      valueOr(d, "distribution", ""),
			"bitFormat":         bf,
			"locale":            valueOr(d, "locale", ""),
			"availableLanguage": langs,
		})
	}
	return sortTemplatesByDistribution(list)
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

// sortTemplatesByDistribution 把 debian / ubuntu / centos 等常用 distro 排前面
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
// body: { templateId: long|string, language?, sshKey?: string[], doNotSendPassword?: bool, softwareId?: long[] }
//
// EU 路径:POST /vps/{name}/reinstall body {templateId: long, sshKey: string[], language, ...}
// US 路径:POST /vps/{name}/rebuild   body {imageId: string,  sshKey: string,   ...}  ← 注意 sshKey 单数
//
// templateId 传入数字 → EU /reinstall;传入字符串 → US /rebuild。前端把 templates 接口返回的 id
// 不加转换直接回传即可,后端根据 JSON 类型自动分路。
func ReinstallVps(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		acc, _ := ovhAccountFor(state, c)
		isUS := acc.Endpoint == "ovh-us"

		var body struct {
			TemplateID        interface{} `json:"templateId"` // long(EU) 或 string(US imageId)
			Language          string      `json:"language"`
			SSHKey            []string    `json:"sshKey"`
			DoNotSendPassword bool        `json:"doNotSendPassword"`
			SoftwareID        []int64     `json:"softwareId"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.TemplateID == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 templateId"})
			return
		}

		if isUS {
			// US: /rebuild + body.imageId (string) + sshKey 单字符串
			imageID := fmt.Sprintf("%v", body.TemplateID)
			params := map[string]interface{}{
				"imageId":           imageID,
				"doNotSendPassword": body.DoNotSendPassword,
				"installRTM":        false,
			}
			if len(body.SSHKey) > 0 {
				params["sshKey"] = body.SSHKey[0] // 取第一个,US 只支持单 key
			}
			var task map[string]interface{}
			if err := client.Post("/vps/"+svc+"/rebuild", params, &task); err != nil {
				state.Logger.Error("VPS "+svc+" rebuild 失败: "+err.Error(), "vps_control")
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
				return
			}
			state.Logger.Info(fmt.Sprintf("VPS %s (US) rebuild 任务已创建: imageId=%s", svc, imageID), "vps_control")
			c.JSON(http.StatusOK, gin.H{"success": true, "message": "重装任务已创建", "task": task})
			return
		}

		// EU: /reinstall + body.templateId (long)
		tid, _ := numconv.ToInt64(body.TemplateID)
		if tid == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "templateId 必须是数字"})
			return
		}
		params := map[string]interface{}{
			"templateId":        tid,
			"doNotSendPassword": body.DoNotSendPassword,
		}
		if body.Language != "" {
			params["language"] = body.Language
		}
		if len(body.SSHKey) > 0 {
			params["sshKey"] = body.SSHKey
		}
		if len(body.SoftwareID) > 0 {
			params["softwareId"] = body.SoftwareID
		}
		var task map[string]interface{}
		if err := client.Post("/vps/"+svc+"/reinstall", params, &task); err != nil {
			state.Logger.Error("VPS "+svc+" reinstall 失败: "+err.Error(), "vps_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("VPS %s reinstall 任务已创建: templateId=%d", svc, tid), "vps_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "重装任务已创建", "task": task})
	}
}

// GetVpsTasks GET /api/vps-control/:service_name/tasks
// /vps/{name}/tasks 返回 long[](taskId 数组),每个 /tasks/{id} 是 vps.Task
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
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		// 只拉最近 10 个
		start := len(ids) - 10
		if start < 0 {
			start = 0
		}
		recent := ids[start:]
		keys := make([]interface{}, len(recent))
		for i, id := range recent {
			keys[i] = id
		}
		details := parallelGetDetails(client, keys, func(k interface{}) string {
			return fmt.Sprintf("/vps/%s/tasks/%v", svc, k)
		}, 10)
		tasks := []gin.H{}
		for i, id := range recent {
			d := details[i]
			if d == nil {
				continue
			}
			progress := 0
			if v, ok := numconv.ToInt64(d["progress"]); ok {
				progress = int(v)
			}
			tasks = append(tasks, gin.H{
				"id":       id,
				"type":     valueOr(d, "type", ""),
				"state":    valueOr(d, "state", "unknown"),
				"date":     valueOr(d, "date", ""),
				"progress": progress,
			})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "tasks": tasks, "total": len(tasks)})
	}
}

// GetVpsTaskDetail GET /api/vps-control/:service_name/tasks/:task_id
// 用于轮询单个任务进度
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
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "task": d})
	}
}
