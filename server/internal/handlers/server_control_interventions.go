package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/ovh"
	"github.com/ovh-webui/server/internal/types"
)

// GetInterventions GET /api/server-control/:service_name/interventions
func GetInterventions(state *app.State) gin.HandlerFunc {
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
		intvPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "intervention")
		if !ok {
			return
		}
		var ids []interface{}
		if err := client.Get(intvPath, &ids); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		// 并发拉详情。失败的条目不能静默丢弃,否则限流/权限问题会让列表凭空变短
		details, detailErrs := hardwareParallelGetDetails(client, ids, func(k interface{}) string {
			return safePathFn("dedicated", "server", svc, "intervention", idToString(k))
		}, 10)
		list := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}
		failed := len(ids) - len(list)
		if len(ids) > 0 && failed == len(ids) {
			fe := hardwareFirstErr(detailErrs)
			state.Logger.Error("获取服务器 "+svc+" 干预记录详情全部失败: "+fe.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": fe.Error()})
			return
		}
		resp := gin.H{"success": true, "interventions": list}
		if failed > 0 {
			fe := hardwareFirstErr(detailErrs)
			state.Logger.Warn(fmt.Sprintf("服务器 %s 干预记录部分获取失败(%d/%d): %s", svc, failed, len(ids), fe.Error()), "server_control")
			resp["partial"] = true
			resp["failed"] = failed
			resp["error"] = "部分干预记录获取失败：" + fe.Error()
		}
		c.JSON(http.StatusOK, resp)
	}
}

// GetInterventionDetail GET /api/server-control/:service_name/interventions/:intervention_id
func GetInterventionDetail(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		id, ok := requirePositiveIDParam(c, "intervention_id", "干预 ID", "INVALID_INTERVENTION_ID")
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		detailPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "intervention", id)
		if !ok {
			return
		}
		var d map[string]interface{}
		if err := client.Get(detailPath, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "intervention": d})
	}
}

// GetPlannedInterventions GET /api/server-control/:service_name/planned-interventions
func GetPlannedInterventions(state *app.State) gin.HandlerFunc {
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
		piPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "plannedIntervention")
		if !ok {
			return
		}
		var ids []interface{}
		if err := client.Get(piPath, &ids); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		// 并发拉详情。同 GetInterventions:失败要显式暴露,不能让列表静默变空
		details, detailErrs := hardwareParallelGetDetails(client, ids, func(k interface{}) string {
			return safePathFn("dedicated", "server", svc, "plannedIntervention", idToString(k))
		}, 10)
		list := []map[string]interface{}{}
		for _, d := range details {
			if d != nil {
				list = append(list, d)
			}
		}
		failed := len(ids) - len(list)
		if len(ids) > 0 && failed == len(ids) {
			fe := hardwareFirstErr(detailErrs)
			state.Logger.Error("获取服务器 "+svc+" 计划干预详情全部失败: "+fe.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": fe.Error()})
			return
		}
		resp := gin.H{"success": true, "plannedInterventions": list}
		if failed > 0 {
			fe := hardwareFirstErr(detailErrs)
			state.Logger.Warn(fmt.Sprintf("服务器 %s 计划干预部分获取失败(%d/%d): %s", svc, failed, len(ids), fe.Error()), "server_control")
			resp["partial"] = true
			resp["failed"] = failed
			resp["error"] = "部分计划干预记录获取失败：" + fe.Error()
		}
		c.JSON(http.StatusOK, resp)
	}
}

// GetPlannedInterventionDetail GET /api/server-control/:service_name/planned-interventions/:intervention_id
func GetPlannedInterventionDetail(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		id, ok := requirePositiveIDParam(c, "intervention_id", "计划干预 ID", "INVALID_INTERVENTION_ID")
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		detailPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "plannedIntervention", id)
		if !ok {
			return
		}
		var d map[string]interface{}
		if err := client.Get(detailPath, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "plannedIntervention": d})
	}
}

// parseReplaceDisks 把请求里的故障盘列表转成 dedicated.server.SupportReplaceHddInfo[]。
// schema: disk_serial 必填 string / slot_id 可空 long。slot_id 拿不到就整个键不发,
// 免得给 OVH 一个 null 槽位号让工单校验失败。
func parseReplaceDisks(raw interface{}) ([]map[string]interface{}, error) {
	if raw == nil {
		return nil, nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("disks 必须是数组")
	}
	out := []map[string]interface{}{}
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("disks[%d] 必须是对象", i)
		}
		serial := ""
		// 前端可能用 snake_case(贴合 OVH schema)或 camelCase,两种都收
		for _, k := range []string{"disk_serial", "diskSerial", "serial"} {
			if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
				serial = strings.TrimSpace(v)
				break
			}
		}
		if serial == "" {
			return nil, fmt.Errorf("disks[%d] 缺少硬盘序列号 disk_serial", i)
		}
		disk := map[string]interface{}{"disk_serial": serial}
		for _, k := range []string{"slot_id", "slotId"} {
			if v, ok := numconv.ToInt64(m[k]); ok {
				disk["slot_id"] = v
				break
			}
		}
		out = append(out, disk)
	}
	return out, nil
}

// parseMemorySlots 把请求里的故障内存槽位转成 schema 要的 string[]。
// 前端可能给数组,也可能给一串逗号分隔的描述,两种都归一化;空值一律丢弃。
func parseMemorySlots(raw interface{}) []string {
	out := []string{}
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
		}
	case string:
		// 全角逗号同样要认,理由见 types.SplitList
		out = append(out, types.SplitList(v)...)
	}
	return out
}

// HardwareReplace POST /api/server-control/:service_name/hardware/replace
func HardwareReplace(state *app.State) gin.HandlerFunc {
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
		var body map[string]interface{}
		_ = c.ShouldBindJSON(&body)
		componentType, _ := body["componentType"].(string)
		comment, _ := body["comment"].(string)
		if componentType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 componentType 参数", "code": "E9871712B"})
			return
		}
		var result map[string]interface{}
		var err2 error
		switch componentType {
		case "hardDiskDrive":
			if comment == "" {
				comment = "Request hard disk drive replacement - faulty disk detected"
			}
			// schema 里 inverse 的语义是"更换所有【未列出】的盘",disks 是"要更换的盘"。
			// 老代码写死 disks:[] + inverse:true,等于向 OVH 申请更换整机每一块硬盘 ——
			// 故障盘必须由调用方显式给出,拿不到就宁可 400 也不发这张工单。
			disks, derr := parseReplaceDisks(body["disks"])
			if derr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": ovh.Explain(derr)})
				return
			}
			if len(disks) == 0 {
				// 两种模式都不能空列表:inverse=false 空 = 没告诉 OVH 换哪块;
				// inverse=true 空 = "未列出的全换" = 整机所有盘。
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "disks 不能为空：正常模式填故障盘序列号；故障盘序列号读不出来时用 inverse=true 并填所有【健康盘】的序列号", "code": "EA0B09D1D",
				})
				return
			}
			// inverse 只有在调用方明确传 true 时才反转语义(即"除这些盘之外全换"),默认 false
			inverse := false
			if v, ok := body["inverse"].(bool); ok {
				inverse = v
			}
			repPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "support", "replace", "hardDiskDrive")
			if !ok {
				return
			}
			err2 = client.Post(repPath, map[string]interface{}{
				"comment": comment,
				"disks":   disks,
				"inverse": inverse,
			}, &result)
		case "memory":
			details := "Memory module failure"
			if v, ok := body["details"].(string); ok && v != "" {
				details = v
			}
			if comment == "" {
				comment = "Request memory module replacement - hardware failure detected"
			}
			// schema 只认 comment / details / slots(string[],可选);slotsDescription 这个字段
			// 在 EU/US schema 里都不存在。可选字段拿不到值就整个不发,不给 OVH 塞未知参数
			// (OVH 是否忽略未知 body 参数没有文档保证,不赌它的宽容度)。
			memoryBody := map[string]interface{}{
				"comment": comment,
				"details": details,
			}
			if slots := parseMemorySlots(body["slots"]); len(slots) > 0 {
				memoryBody["slots"] = slots
			}
			repPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "support", "replace", "memory")
			if !ok {
				return
			}
			err2 = client.Post(repPath, memoryBody, &result)
		case "cooling":
			details := "Cooling system failure"
			if v, ok := body["details"].(string); ok && v != "" {
				details = v
			}
			if comment == "" {
				comment = "Request cooling system replacement - fan failure or overheating"
			}
			repPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "support", "replace", "cooling")
			if !ok {
				return
			}
			err2 = client.Post(repPath, map[string]interface{}{
				"comment": comment,
				"details": details,
			}, &result)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "不支持的组件类型: " + componentType})
			return
		}
		if err2 != nil {
			errMsg := ovh.Explain(err2)
			state.Logger.Error("硬件更换失败: "+svc+" - "+componentType+" - "+errMsg, "server_control")
			if strings.Contains(errMsg, "Action pending") {
				ticketID := "未知"
				if m := regexp.MustCompile(`ticketId[:\s]+(\d+)`).FindStringSubmatch(errMsg); m != nil {
					ticketID = m[1]
				}
				c.JSON(http.StatusBadRequest, gin.H{
					"success":   false,
					"error":     "已有待处理的硬件更换工单 (Ticket #" + ticketID + ")，请等待完成后再提交新请求",
					"ticketId":  ticketID,
					"isPending": true,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": errMsg})
			return
		}
		// 响应是 support.NewMessageInfo:ticketId / ticketNumber / messageId / additionalNotice。
		// 工单号必须回给用户 —— OVH 的整个更换流程是工单制的,拿不到号就没法去帮助中心跟进
		ticketNumber := numconv.ToString(result["ticketNumber"])
		ticketID := numconv.ToString(result["ticketId"])
		notice, _ := result["additionalNotice"].(string)
		msg := "硬件更换工单已提交"
		if ticketNumber != "" && ticketNumber != "0" {
			msg += "，工单号 #" + ticketNumber
		} else if ticketID != "" && ticketID != "0" {
			msg += "，工单 ID " + ticketID
		}
		state.Logger.Info("硬件更换请求已发送: "+svc+" - "+componentType+" - "+msg, "server_control")
		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"message":      msg,
			"ticketNumber": ticketNumber,
			"ticketId":     ticketID,
			"notice":       notice,
			"task":         result,
		})
	}
}
