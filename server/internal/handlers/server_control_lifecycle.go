package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
)

// Ongoing
func GetOngoingTasks(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "ongoing")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "ongoing": d})
	}
}

// License
//
// ⚠️ 区域提醒：compliantWindows / compliantWindowsSqlServer 这两条【只读】端点 EU/US/CA 三区都有，
// 所以不做门控；但真正下单激活的 POST /dedicated/server/{sn}/license/windows 只有 EU 与 CA 有，
// US 站点整个 dedicated/server 命名空间里没有它(live 三站 schema 已核实)。
// 也就是说美区账户查得到"兼容哪些 Windows 版本"，却无法通过 API 买许可证。
// 将来若在这里补上"订购 Windows 许可证"的 handler，必须先用 srvRegionFor 挡掉 US，
// 否则美区用户会点到一个必然 404 的按钮。
func GetCompliantWindowsVersions(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "license", "compliantWindows")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "versions": d})
	}
}

func GetCompliantWindowsSqlVersions(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "license", "compliantWindowsSqlServer")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "versions": d})
	}
}

// Termination

// terminationReasons / terminationFutureUses 对应 schema 的 service.TerminationReasonEnum
// 与 service.TerminationFutureUseEnum,用于确认终止时的可选参数校验。
var terminationReasons = map[string]bool{
	"FEATURES_DONT_SUIT_ME":           true,
	"LACK_OF_PERFORMANCES":            true,
	"MIGRATED_TO_ANOTHER_OVH_PRODUCT": true,
	"MIGRATED_TO_COMPETITOR":          true,
	"NOT_ENOUGH_RECOGNITION":          true,
	"NOT_NEEDED_ANYMORE":              true,
	"NOT_RELIABLE":                    true,
	"NO_ANSWER":                       true,
	"OTHER":                           true,
	"PRODUCT_DIMENSION_DONT_SUIT_ME":  true,
	"PRODUCT_TOOLS_DONT_SUIT_ME":      true,
	"TOO_EXPENSIVE":                   true,
	"TOO_HARD_TO_USE":                 true,
	"UNSATIFIED_BY_CUSTOMER_SUPPORT":  true,
}

var terminationFutureUses = map[string]bool{
	"NOT_REPLACING_SERVICE":      true,
	"OTHER":                      true,
	"SUBSCRIBE_AN_OTHER_SERVICE": true,
	"SUBSCRIBE_OTHER_KIND_OF_SERVICE_WITH_COMPETITOR": true,
	"SUBSCRIBE_SIMILAR_SERVICE_WITH_COMPETITOR":       true,
}

// TerminateService ⚠️ 立即终止。
//
// 实测(2026-08-25):这条链路走完之后 OVH **当场把服务器暂停**,并邮件通知
// 「5 天内不付款就彻底清除硬盘数据,此操作无法撤销」—— 不是等到期日。
//
// 想要「到期才终止」请用 UpdateTerminationPolicy(PUT /services/{serviceId} 的
// terminationPolicy = terminateAtExpirationDate)。OVH 的生命周期动作枚举里
// terminate 和 terminateAtExpirationDate 是两个不同的动作,这个端点只对应前者。
func TerminateService(state *app.State) gin.HandlerFunc {
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
		termPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "terminate")
		if !ok {
			return
		}
		var resp string
		if err := client.Post(termPath, map[string]interface{}{}, &resp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Warn("服务器 "+svc+" 终止请求已提交, token 已发送至管理员邮箱", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "终止请求已提交,token 已发送至管理员邮箱,请查收邮件后再确认终止", "code": "E1A39F5C5", "response": resp})
	}
}

func ConfirmTermination(state *app.State) gin.HandlerFunc {
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
		var body struct {
			Token      string `json:"token"`
			Reason     string `json:"reason"`
			FutureUse  string `json:"futureUse"`
			Commentary string `json:"commentary"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少token参数", "code": "ED4D892CA"})
			return
		}
		if body.Reason != "" && !terminationReasons[body.Reason] {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "无效的终止原因: " + body.Reason})
			return
		}
		if body.FutureUse != "" && !terminationFutureUses[body.FutureUse] {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "无效的后续用途: " + body.FutureUse})
			return
		}
		payload := map[string]interface{}{"token": body.Token}
		if body.Reason != "" {
			payload["reason"] = body.Reason
		}
		if body.FutureUse != "" {
			payload["futureUse"] = body.FutureUse
		}
		if body.Commentary != "" {
			payload["commentary"] = body.Commentary
		}
		confirmPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "confirmTermination")
		if !ok {
			return
		}
		var resp string
		if err := client.Post(confirmPath, payload, &resp); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Warn("服务器 "+svc+" 终止已确认", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "终止已确认", "code": "EC265A0A4"})
	}
}

// SPLA
func GetSPLAList(state *app.State) gin.HandlerFunc {
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
		splaPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "spla")
		if !ok {
			return
		}
		var ids []interface{}
		if err := client.Get(splaPath, &ids); err != nil {
			state.Logger.Error("获取SPLA列表失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		details, detailErrs := miscParallelGetDetails(client, ids, func(k interface{}) string {
			return safePathFn("dedicated", "server", svc, "spla", idToString(k))
		}, 10)
		list, failed := miscMergeDetails(ids, "id", details, detailErrs)
		if failed > 0 {
			state.Logger.Warn(fmt.Sprintf("服务器 %s SPLA 详情有 %d/%d 项拉取失败", svc, failed, len(ids)), "server_control")
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "splaList": list, "partial": failed > 0, "failedCount": failed})
	}
}

func CreateSPLA(state *app.State) gin.HandlerFunc {
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
		var body struct {
			Type         string `json:"type"`
			SerialNumber string `json:"serialNumber"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.Type == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少type参数", "code": "E47BAC7E6"})
			return
		}
		if body.Type != "os" && body.Type != "sqlstd" && body.Type != "sqlweb" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "无效的许可证类型: " + body.Type + " (只支持 os / sqlstd / sqlweb)"})
			return
		}
		if body.SerialNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少serialNumber参数", "code": "E9AF18D58"})
			return
		}
		splaPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "spla")
		if !ok {
			return
		}
		payload := map[string]interface{}{"type": body.Type, "serialNumber": body.SerialNumber}
		var newID int64
		if err := client.Post(splaPath, payload, &newID); err != nil {
			state.Logger.Error("创建SPLA许可证失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info(fmt.Sprintf("创建SPLA许可证成功: %s, id=%d", body.Type, newID), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "SPLA许可证已创建", "code": "E7229383E", "id": newID})
	}
}

// BIOS
func GetBIOSSettings(state *app.State) gin.HandlerFunc {
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
		biosPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "biosSettings")
		if !ok {
			return
		}
		state.Logger.Info("[BIOS] 获取服务器 "+svc+" BIOS 设置", "server_control")
		var d map[string]interface{}
		if err := client.Get(biosPath, &d); err != nil {
			msg := err.Error()
			if ovhIsNotFound(err) {
				state.Logger.Warn("[BIOS] 服务器 "+svc+" 不支持 BIOS 设置: "+msg, "server_control")
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "BIOS 设置不可用", "code": "E2995BE84"})
				return
			}
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": msg})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "bios": d})
	}
}

func GetBIOSSettingsSGX(state *app.State) gin.HandlerFunc {
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
		sgxPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "biosSettings", "sgx")
		if !ok {
			return
		}
		state.Logger.Info("[BIOS] 获取服务器 "+svc+" SGX BIOS 设置", "server_control")
		var d map[string]interface{}
		if err := client.Get(sgxPath, &d); err != nil {
			msg := err.Error()
			// 同上:只认 404,别把权限/锁定错误误报成"SGX 不可用"
			if ovhIsNotFound(err) {
				state.Logger.Warn("[BIOS] 服务器 "+svc+" 不支持 SGX: "+msg, "server_control")
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "SGX 不可用", "code": "EACCA3920"})
				return
			}
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": msg})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "sgx": d})
	}
}

func intToStr(v int64) string {
	return formatInt(v)
}

func formatInt(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func containsAny(s string, subs []string) bool {
	lc := lower(s)
	for _, sub := range subs {
		if indexOf(lc, lower(sub)) >= 0 {
			return true
		}
	}
	return false
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
