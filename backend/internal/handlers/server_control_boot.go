package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
)

// GetBootConfig GET /api/server-control/:service_name/boot
func GetBootConfig(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var info map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc, &info); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		bootID := info["bootId"]
		bootIDInt, _ := numconv.ToInt64(bootID)
		var bootList []int64
		_ = client.Get("/dedicated/server/"+svc+"/boot", &bootList)
		// 并发拉每个 boot id 的详情
		details := make([]map[string]interface{}, len(bootList))
		sem := make(chan struct{}, 10)
		var wg sync.WaitGroup
		for i, bid := range bootList {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, b int64) {
				defer wg.Done()
				defer func() { <-sem }()
				var d map[string]interface{}
				if err := client.Get(fmt.Sprintf("/dedicated/server/%s/boot/%d", svc, b), &d); err == nil {
					details[idx] = d
				}
			}(i, bid)
		}
		wg.Wait()

		boots := []gin.H{}
		for i, bid := range bootList {
			detail := details[i]
			if detail == nil {
				continue
			}
			isCurrent := bootIDInt == bid
			boots = append(boots, gin.H{
				"id":          bid,
				"bootType":    valueOr(detail, "bootType", "N/A"),
				"description": valueOr(detail, "description", ""),
				"kernel":      valueOr(detail, "kernel", ""),
				"isCurrent":   isCurrent,
			})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "currentBootId": bootID, "boots": boots})
	}
}

// SetBootConfig PUT /api/server-control/:service_name/boot/:boot_id
func SetBootConfig(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		bootIDStr := c.Param("boot_id")
		// Python 路由 <int:boot_id> 强制转 int，OVH API bootId 字段也要求整数
		// 之前 Go 把字符串直接塞进 body 会被 OVH 拒
		bootID, err := strconv.ParseInt(bootIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "boot_id 必须是整数"})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		if err := client.Put("/dedicated/server/"+svc, map[string]interface{}{
			"bootId": bootID,
		}, nil); err != nil {
			state.Logger.Error("设置服务器 "+svc+" 启动模式失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info(fmt.Sprintf("服务器 %s 启动模式已设置为 %d", svc, bootID), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "启动模式已更新，重启后生效"})
	}
}

// GetMonitoringStatus GET /api/server-control/:service_name/monitoring
func GetMonitoringStatus(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var info map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc, &info); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		// 1:1 对应 Python app.py:6111：缺失时默认 false
		monitoring := info["monitoring"]
		if monitoring == nil {
			monitoring = false
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "monitoring": monitoring})
	}
}

// SetMonitoringStatus PUT /api/server-control/:service_name/monitoring
func SetMonitoringStatus(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			Enabled bool `json:"enabled"`
		}
		_ = c.ShouldBindJSON(&body)
		if err := client.Put("/dedicated/server/"+svc, map[string]interface{}{
			"monitoring": body.Enabled,
		}, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		statusText := "开启"
		if !body.Enabled {
			statusText = "关闭"
		}
		state.Logger.Info("服务器 "+svc+" 监控已"+statusText, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "监控已" + statusText})
	}
}

// GetBootModes GET /api/server-control/:service_name/boot-mode
func GetBootModes(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var info map[string]interface{}
		if err := client.Get("/dedicated/server/"+svc, &info); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		currentBootID := info["bootId"]
		currentBootIDInt, _ := numconv.ToInt64(currentBootID)
		var bootIDs []int64
		_ = client.Get("/dedicated/server/"+svc+"/boot", &bootIDs)
		// 并发拉每个 boot id 详情
		biDetails := make([]map[string]interface{}, len(bootIDs))
		bSem := make(chan struct{}, 10)
		var bWg sync.WaitGroup
		for i, bid := range bootIDs {
			bWg.Add(1)
			bSem <- struct{}{}
			go func(idx int, b int64) {
				defer bWg.Done()
				defer func() { <-bSem }()
				var bi map[string]interface{}
				if err := client.Get(fmt.Sprintf("/dedicated/server/%s/boot/%d", svc, b), &bi); err == nil {
					biDetails[idx] = bi
				}
			}(i, bid)
		}
		bWg.Wait()

		modes := []gin.H{}
		for i, bid := range bootIDs {
			bi := biDetails[i]
			if bi == nil {
				continue
			}
			active := currentBootIDInt == bid
			modes = append(modes, gin.H{
				"id":          bid,
				"bootType":    bi["bootType"],
				"description": bi["description"],
				"kernel":      bi["kernel"],
				"active":      active,
			})
		}
		state.Logger.Info(fmt.Sprintf("[Boot] 找到 %d 个启动模式", len(modes)), "server_control")
		c.JSON(http.StatusOK, gin.H{
			"success":       true,
			"currentBootId": currentBootID,
			"bootModes":     modes,
		})
	}
}

// ChangeBootMode PUT /api/server-control/:service_name/boot-mode
func ChangeBootMode(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc := c.Param("service_name")
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		var body struct {
			BootID int64 `json:"bootId"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.BootID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少bootId参数"})
			return
		}
		state.Logger.Info(fmt.Sprintf("[Boot] 切换服务器 %s 启动模式到 %d", svc, body.BootID), "server_control")
		if err := client.Put("/dedicated/server/"+svc, map[string]interface{}{
			"bootId": body.BootID,
		}, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
			return
		}
		state.Logger.Info("[Boot] 启动模式切换成功，需要重启服务器生效", "server_control")
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "启动模式已切换，需要重启服务器生效",
			"bootId":  body.BootID,
		})
	}
}
