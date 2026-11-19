package handlers

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// ipmiAccessPriority 自动挑选时的优先级：HTML5 KVM 最通用（浏览器直接开），
// 其次 Java KVM（JNLP，需要 Java Web Start），再退到 Serial-over-LAN(URL)，
// 最后才是需要 SSH 公钥的 Serial-over-LAN(SSH)。
// 四个值都在 dedicated.server.IpmiAccessTypeEnum 里。
//
// 注意这只是**没指定类型时**的兜底顺序。调用方可以用 ?type= 显式指定 ——
// 有些机型的 HTML5 KVM 有兼容问题（键盘映射、鼠标不同步），老运维就是要 Java KVM，
// 自动挑选把 HTML5 排第一会让他们永远拿不到 JNLP。
var ipmiAccessPriority = []string{"kvmipHtml5URL", "kvmipJnlp", "serialOverLanURL", "serialOverLanSshKey"}

// ipmiAccessTypes dedicated.server.IpmiAccessTypeEnum 的全部合法取值
var ipmiAccessTypes = map[string]string{
	"kvmipHtml5URL":       "HTML5 KVM（浏览器直接打开）",
	"kvmipJnlp":           "Java KVM（下载 .jnlp，需要 Java Web Start）",
	"serialOverLanURL":    "串口重定向 SOL（浏览器打开）",
	"serialOverLanSshKey": "串口重定向 SOL（SSH，需要公钥）",
}

// GetIPMIAccessTypes GET /api/server-control/:service_name/ipmi-types
//
// 只查这台机器支持哪几种控制台接入方式,不申请会话。
// 单独开这个接口是因为申请会话要轮询 OVH 任务、动辄 20 秒 ——
// 让用户等 20 秒才知道"原来这台机器没有 Java KVM"是很糟的体验。
func GetIPMIAccessTypes(state *app.State) gin.HandlerFunc {
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
		ipmiPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "ipmi")
		if !ok {
			return
		}
		var ipmi map[string]interface{}
		if err := client.Get(ipmiPath, &ipmi); err != nil {
			if ovhIsNotFound(err) {
				c.JSON(http.StatusOK, gin.H{
					"success":        true,
					"supportedTypes": []string{},
					"message":        "该服务器没有 IPMI 功能", "code": "E5A05B624",
				})
				return
			}
			state.Logger.Error("[IPMI] 查询支持的控制台类型失败: "+err.Error(), "server_control")
			respondOVHError(c, err)
			return
		}
		supportedList := []string{}
		if sf, ok := ipmi["supportedFeatures"].(map[string]interface{}); ok {
			for _, t := range ipmiAccessPriority {
				if b, _ := sf[t].(bool); b {
					supportedList = append(supportedList, t)
				}
			}
		}
		activated := true
		if v, ok := ipmi["activated"].(bool); ok {
			activated = v
		}
		c.JSON(http.StatusOK, gin.H{
			"success":        true,
			"supportedTypes": supportedList,
			"typeLabels":     ipmiAccessTypes,
			"activated":      activated,
			// 没指定 type 时后端会挑哪个,前端拿它当默认选中项
			"defaultType": func() string {
				if len(supportedList) > 0 {
					return supportedList[0]
				}
				return ""
			}(),
		})
	}
}

// IPMI Console
func GetIPMIConsole(state *app.State) gin.HandlerFunc {
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
		ipmiPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "ipmi")
		if !ok {
			return
		}
		state.Logger.Info("[IPMI] 获取服务器 "+svc+" IPMI信息", "server_control")
		var ipmi map[string]interface{}
		if err := client.Get(ipmiPath, &ipmi); err != nil {
			respondOVHError(c, err)
			return
		}
		// dedicated.server.Ipmi.activated 是必填 boolean，但 schema 没说清它是「IPMI 功能可用」
		// 还是「当前有活动会话」。之前在这里直接 400 会把部分机型原本能开的控制台堵死
		// 该字段以前从不检查，所以只记一笔、并在 OVH 真的拒绝时把它当成提示补上。
		ipmiActivated := true
		if v, ok := ipmi["activated"].(bool); ok {
			ipmiActivated = v
		}
		if !ipmiActivated {
			state.Logger.Warn("[IPMI] 服务器 "+svc+" 的 activated=false，仍尝试申请控制台访问", "server_control")
		}
		// dedicated.server.IpmiSupportedFeatures 的四个字段都是「必填 boolean」，
		// OVH 总把 4 个 key 全返回(值为 true/false)。老代码判的是「key 是否存在」，
		// 于是永远命中第一个分支 kvmipHtml5URL，只支持 SOL 的老机型会被强行要 HTML5 KVM。
		// 先算出这台机器到底支持哪几种接入方式,后面无论自动挑还是用户指定都要用
		supported := map[string]bool{}
		supportedList := []string{}
		if sf, ok := ipmi["supportedFeatures"].(map[string]interface{}); ok {
			for _, t := range ipmiAccessPriority {
				if b, _ := sf[t].(bool); b {
					supported[t] = true
					supportedList = append(supportedList, t)
				}
			}
		}

		var accessType string
		// ?type= 显式指定(例如老运维就要 Java KVM),校验两道:枚举合法 + 这台机器确实支持
		if want := strings.TrimSpace(c.Query("type")); want != "" {
			if _, ok := ipmiAccessTypes[want]; !ok {
				c.JSON(http.StatusBadRequest, gin.H{
					"success":    false,
					"error":      "不支持的控制台类型: " + want,
					"validTypes": ipmiAccessTypes,
				})
				return
			}
			if !supported[want] {
				c.JSON(http.StatusBadRequest, gin.H{
					"success":        false,
					"error":          "该服务器不支持「" + ipmiAccessTypes[want] + "」",
					"supportedTypes": supportedList,
				})
				return
			}
			accessType = want
		} else {
			// 没指定 → 按优先级自动挑
			for _, t := range ipmiAccessPriority {
				if supported[t] {
					accessType = t
					break
				}
			}
		}
		if accessType == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "服务器不支持KVM控制台访问", "code": "EF2514B25"})
			return
		}
		state.Logger.Info("[IPMI] 请求KVM控制台访问，类型: "+accessType, "server_control")
		clientIP := strings.TrimSpace(c.ClientIP())
		params := map[string]interface{}{
			"type": accessType,
			// ttl 是 dedicated.server.CacheTTLEnum(enumType=long)，合法值 1/3/5/10/15，15 是上限
			"ttl": 15,
		}
		if parsed := net.ParseIP(clientIP); parsed != nil && parsed.To4() != nil &&
			!parsed.IsLoopback() && !parsed.IsPrivate() && !parsed.IsLinkLocalUnicast() && !parsed.IsUnspecified() {
			normalized := parsed.To4().String()
			params["ipToAllow"] = normalized
			state.Logger.Info("[IPMI] 添加IP白名单: "+normalized, "server_control")
		} else {
			state.Logger.Warn("[IPMI] 跳过IP白名单（非公网 IPv4 地址: "+clientIP+"），本次控制台会话不限制来源 IP", "server_control")
		}
		if sshKey := strings.TrimSpace(c.Query("sshKey")); sshKey != "" {
			params["sshKey"] = sshKey
		}
		accessPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "features", "ipmi", "access")
		if !ok {
			return
		}
		var task map[string]interface{}
		if err := client.Post(accessPath, params, &task); err != nil {
			msg := ovh.Explain(err)
			if !ipmiActivated {
				msg += "（该服务器的 IPMI activated=false，很可能需要先在 OVH 管理后台启用 IPMI）"
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": msg})
			return
		}
		taskID := task["taskId"]
		taskIDStr := strings.TrimSpace(idToString(taskID))
		taskPath, pErr := buildOVHPath("dedicated", "server", svc, "task", taskIDStr)
		if pErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "OVH 返回无效的 IPMI 任务 ID"})
			return
		}
		state.Logger.Info(fmt.Sprintf("[IPMI] 创建访问任务: taskId=%v, status=%v", taskID, task["status"]), "server_control")
		// 轮询
		maxRetries := 10
		taskCompleted := false
		for i := 0; i < maxRetries; i++ {
			time.Sleep(2 * time.Second)
			var ts map[string]interface{}
			// OVH 错误直接返回 500，
			// 之前 Go 静默 continue 会掩盖 OVH 真错误，最终用 "超时" 假面具吞掉
			if err := client.Get(taskPath, &ts); err != nil {
				state.Logger.Error(fmt.Sprintf("[IPMI] 查询任务 %v 状态失败: %s", taskID, err.Error()), "server_control")
				respondOVHError(c, err)
				return
			}
			status, _ := ts["status"].(string)
			state.Logger.Info(fmt.Sprintf("[IPMI] 任务状态检查 (%d/%d): %s", i+1, maxRetries, status), "server_control")
			if status == "done" {
				taskCompleted = true
				break
			}
			if status == "cancelled" || status == "customerError" || status == "ovhError" {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "IPMI访问任务失败: " + status})
				return
			}
		}
		if !taskCompleted {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "IPMI访问任务超时", "code": "E8953934A"})
			return
		}
		consolePath, ok := ovhPathQueryOrAbort(c, url.Values{"type": {accessType}}, "dedicated", "server", svc, "features", "ipmi", "access")
		if !ok {
			return
		}
		var consoleAccess map[string]interface{}
		// 这一步的错误以前被 `_ =` 吞掉，前端只能拿到 success:true + console:null，日志里也查不到原因
		if err := client.Get(consolePath, &consoleAccess); err != nil {
			state.Logger.Error("[IPMI] 获取控制台地址失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "获取控制台地址失败: " + ovh.Explain(err)})
			return
		}
		// dedicated.server.IpmiAccessValue.value 是「可空」string：任务 done 了 OVH 也可能还没生成地址
		if v, _ := consoleAccess["value"].(string); v == "" {
			state.Logger.Error(fmt.Sprintf("[IPMI] OVH 返回空控制台地址: type=%s, resp=%v", accessType, consoleAccess), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "OVH 未返回控制台地址，请稍后重试", "code": "E35CB5452"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"ipmi":       ipmi,
			"console":    consoleAccess,
			"accessType": accessType,
			// 告诉前端这台机器还支持哪些方式,好让用户换一种再开
			"supportedTypes": supportedList,
			"typeLabels":     ipmiAccessTypes,
		})
	}
}
