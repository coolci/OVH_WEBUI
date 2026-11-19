package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/ovh"
)

// GetServerTasks GET /api/server-control/:service_name/tasks
func GetServerTasks(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		taskListPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "task")
		if !ok {
			return
		}
		var taskIDs []interface{}
		if err := client.Get(taskListPath, &taskIDs); err != nil {
			state.Logger.Error("获取服务器 "+svc+" 任务列表失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		// schema 只承诺返回 long[]、没承诺顺序，所以先自己按 id 升序排再取最近 10 个
		sort.SliceStable(taskIDs, func(i, j int) bool {
			a, _ := numconv.ToInt64(taskIDs[i])
			b, _ := numconv.ToInt64(taskIDs[j])
			return a < b
		})
		start := len(taskIDs) - 10
		if start < 0 {
			start = 0
		}
		recent := taskIDs[start:]

		// 详情失败不能静默丢条目：并发拉详情遇到限流/超时时列表会莫名其妙变短，
		// 用户看不到刚触发的任务就会重复提交，这里保留占位并把错误带给前端
		details := make([]map[string]interface{}, len(recent))
		detailErrs := make([]error, len(recent))
		sem := make(chan struct{}, 10)
		var wg sync.WaitGroup
		for i, tid := range recent {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, id interface{}) {
				defer wg.Done()
				defer func() { <-sem }()
				var d map[string]interface{}
				client, err := ovh.IsolateClient(client)
				if err != nil {
					detailErrs[idx] = err
					return
				}
				tPath, tErr := buildOVHPath("dedicated", "server", svc, "task", idToString(id))
				if tErr != nil {
					detailErrs[idx] = tErr
					return
				}
				if err := client.Get(tPath, &d); err != nil {
					detailErrs[idx] = err
					return
				}
				details[idx] = d
			}(i, tid)
		}
		wg.Wait()

		tasks := []gin.H{}
		failed := 0
		for i, taskID := range recent {
			detail := details[i]
			if detail == nil {
				failed++
				msg := "任务详情获取失败"
				if detailErrs[i] != nil {
					msg = detailErrs[i].Error()
				}
				state.Logger.Error(fmt.Sprintf("获取服务器 %s 任务 %s 详情失败: %s", svc, idToString(taskID), msg), "server_control")
				tasks = append(tasks, gin.H{
					"taskId":    taskID,
					"function":  "N/A",
					"status":    "unknown",
					"comment":   "",
					"startDate": "",
					"doneDate":  "",
					"error":     msg,
				})
				continue
			}
			tasks = append(tasks, gin.H{
				"taskId":    taskID,
				"function":  valueOr(detail, "function", "N/A"),
				"status":    valueOr(detail, "status", "unknown"),
				"comment":   valueOr(detail, "comment", ""),
				"startDate": valueOr(detail, "startDate", ""),
				"doneDate":  valueOr(detail, "doneDate", ""),
			})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "tasks": tasks, "total": len(tasks), "failed": failed})
	}
}

// normalizeAPIDate 把入参归一成 OVH 的 date 类型(YYYY-MM-DD)。
// 接受纯日期和完整 ISO8601 时间戳两种写法，其余一律拒掉：
// schema 写死了参数是 date，与其赌 OVH 解析宽松，不如在这里就把格式错误讲清楚
func normalizeAPIDate(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("2006-01-02"), true
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		// 取时间戳自带偏移下的那一天，不折算成 UTC：
		// 2026-08-24T23:00:00-05:00 在调用方眼里就是 8/24，折成 UTC 会整体偏一天，查询窗口跟着错位
		return t.Format("2006-01-02"), true
	}
	return "", false
}

// GetTaskAvailableTimeslots GET /api/server-control/:service_name/tasks/:task_id/available-timeslots
func GetTaskAvailableTimeslots(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		taskID, ok := requirePositiveIDParam(c, "task_id", "任务 ID", "INVALID_TASK_ID")
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		periodStart, ok := normalizeAPIDate(c.Query("periodStart"))
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "periodStart 必须是 YYYY-MM-DD 或 ISO8601 时间", "code": "E36D84314"})
			return
		}
		periodEnd, ok := normalizeAPIDate(c.Query("periodEnd"))
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "periodEnd 必须是 YYYY-MM-DD 或 ISO8601 时间", "code": "E6E5E89F8"})
			return
		}
		state.Logger.Info(fmt.Sprintf("[Task] 查询任务 %s 的可用时间段 %s -> %s", taskID, periodStart, periodEnd), "server_control")
		var slots []map[string]interface{}
		q := url.Values{}
		q.Set("periodStart", periodStart)
		q.Set("periodEnd", periodEnd)
		path, ok := ovhPathQueryOrAbort(c, q, "dedicated", "server", svc, "task", taskID, "availableTimeslots")
		if !ok {
			return
		}
		if err := client.Get(path, &slots); err != nil {
			var apiErr *ovhsdk.APIError
			if errors.As(err, &apiErr) {
				// "no schedule needed" 是 OVH 对不需要预约的任务给的业务语义，
				// 只认这条具体消息，不拿整条 error 字符串模糊匹配
				if strings.Contains(strings.ToLower(apiErr.Message), "no schedule needed") {
					state.Logger.Info("[Task] 任务无需预约: "+apiErr.Message, "server_control")
					c.JSON(http.StatusOK, gin.H{
						"success":             true,
						"timeslots":           []interface{}{},
						"scheduleNotRequired": true,
						"message":             "该任务无需预约", "code": "E4F64261F",
					})
					return
				}
				if apiErr.Code == http.StatusNotFound {
					state.Logger.Warn("[Task] 任务或服务器不存在: "+err.Error(), "server_control")
					c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "任务或服务器不存在", "code": "ECE34F4F3"})
					return
				}
			}
			state.Logger.Error("[Task] 可用时间段API错误: "+err.Error(), "server_control")
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		if slots == nil {
			slots = []map[string]interface{}{}
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "timeslots": slots})
	}
}

// ScheduleTaskTimeslot POST /api/server-control/:service_name/tasks/:task_id/schedule
func ScheduleTaskTimeslot(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		taskID, ok := requirePositiveIDParam(c, "task_id", "任务 ID", "INVALID_TASK_ID")
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		// schema: POST .../schedule 的 body 只有 wantedBeginingDate(datetime) 和
		// hasPerformedBackup(boolean) 两个必填参数，之前发的 startDate/endDate 必被 400。
		// startDate 留作兼容别名，免得已有调用方直接断链
		var body struct {
			WantedBeginingDate string `json:"wantedBeginingDate"`
			StartDate          string `json:"startDate"`
			HasPerformedBackup *bool  `json:"hasPerformedBackup"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请求体格式错误: " + ovh.Explain(err)})
			return
		}
		wanted := strings.TrimSpace(body.WantedBeginingDate)
		if wanted == "" {
			wanted = strings.TrimSpace(body.StartDate)
		}
		if wanted == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 wantedBeginingDate (ISO8601 日期时间)", "code": "E21D1E278"})
			return
		}
		wantedTime, perr := time.Parse(time.RFC3339, wanted)
		if perr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "wantedBeginingDate 必须是 ISO8601 日期时间，例如 2026-01-02T15:04:05Z", "code": "E6FD24026"})
			return
		}
		// hasPerformedBackup 是 schema 必填项，缺了 OVH 必 400，所以在这里就要求调用方显式表态。
		// 但 false 在 schema 里是合法取值（它是"你是否已备份"的如实回答，不是开关），
		// 上一轮直接 400 拒掉等于把 OVH 允许的调用变成不可达，这里改成原样透传 + 记一条警告日志。
		if body.HasPerformedBackup == nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 hasPerformedBackup：预约干预前必须确认是否已备份数据", "code": "E9222DCA1"})
			return
		}
		if !*body.HasPerformedBackup {
			state.Logger.Warn(fmt.Sprintf("[Task] 任务 %s 预约干预时声明未备份数据(hasPerformedBackup=false)，干预可能导致数据丢失", taskID), "server_control")
		}
		state.Logger.Info(fmt.Sprintf("[Task] 预约任务 %s 干预开始时间 %s", taskID, wantedTime.Format(time.RFC3339)), "server_control")
		path, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "task", taskID, "schedule")
		if !ok {
			return
		}
		// 该端点返回 void，不给 resType，免得 go-ovh 去解析一个空 body
		if err := client.Post(path, map[string]interface{}{
			"wantedBeginingDate": wantedTime.Format(time.RFC3339),
			"hasPerformedBackup": *body.HasPerformedBackup,
		}, nil); err != nil {
			var apiErr *ovhsdk.APIError
			if errors.As(err, &apiErr) {
				switch apiErr.Code {
				case http.StatusNotFound:
					state.Logger.Warn("[Task] 任务或服务器不存在: "+err.Error(), "server_control")
					c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "任务或服务器不存在", "code": "ECE34F4F3"})
					return
				case http.StatusBadRequest, http.StatusConflict:
					// OVH 的业务校验（任务不支持预约、时间段已被占用等）原样透出
					state.Logger.Error("[Task] 预约任务被拒绝: "+err.Error(), "server_control")
					c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": apiErr.Message})
					return
				}
			}
			state.Logger.Error("[Task] 预约任务API错误: "+err.Error(), "server_control")
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info(fmt.Sprintf("[Task] 任务 %s 干预时间预约成功", taskID), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "干预时间已预约", "code": "E98AC2C1C"})
	}
}
