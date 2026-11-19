package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
	"strings"
)

// miscParallelGetStringKeys 与 util.go 的 parallelGetStringKeys 同构,额外把每项的错误带出来。
// 详情拉取失败时列表项只剩主键(状态/IP/到期全空),不把错误交出去的话前端无法区分
// "OVH 没返回这些字段" 和 "这次没拉到",只能把残缺数据当真实状态显示。
func miscParallelGetStringKeys(client *ovhsdk.Client, keys []string, pathFn func(string) string, concurrency int) ([]map[string]interface{}, []error) {
	if concurrency <= 0 {
		concurrency = 10
	}
	results := make([]map[string]interface{}, len(keys))
	errs := make([]error, len(keys))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, k := range keys {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, key string) {
			defer wg.Done()
			defer func() { <-sem }()
			var d map[string]interface{}
			client, err := ovh.IsolateClient(client)
			if err != nil {
				errs[idx] = err
				return
			}
			p := pathFn(key)
			if p == "" {
				errs[idx] = errUnsafeOVHSegment
				return
			}
			if err := client.Get(p, &d); err != nil {
				errs[idx] = err
				return
			}
			results[idx] = d
		}(i, k)
	}
	wg.Wait()
	return results, errs
}

// miscParallelGetDetails 同上,给主键是数字 ID(interface{})的列表用。
func miscParallelGetDetails(client *ovhsdk.Client, keys []interface{}, pathFn func(interface{}) string, concurrency int) ([]map[string]interface{}, []error) {
	if concurrency <= 0 {
		concurrency = 10
	}
	results := make([]map[string]interface{}, len(keys))
	errs := make([]error, len(keys))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i, k := range keys {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, key interface{}) {
			defer wg.Done()
			defer func() { <-sem }()
			var d map[string]interface{}
			client, err := ovh.IsolateClient(client)
			if err != nil {
				errs[idx] = err
				return
			}
			p := pathFn(key)
			if p == "" {
				errs[idx] = errUnsafeOVHSegment
				return
			}
			if err := client.Get(p, &d); err != nil {
				errs[idx] = err
				return
			}
			results[idx] = d
		}(i, k)
	}
	wg.Wait()
	return results, errs
}

// miscKeysAsIface 把 string 主键列表转成 interface{} 列表,便于和 miscMergeDetails 复用同一段组装逻辑。
func miscKeysAsIface(keys []string) []interface{} {
	out := make([]interface{}, len(keys))
	for i, k := range keys {
		out[i] = k
	}
	return out
}

// miscMergeDetails 把主键列表和并发拉到的详情合并成前端列表,并统计失败项。
// 失败项仍然保留(用户至少知道这条资源存在),但带上 _detailError 说明这一行是残缺的。
func miscMergeDetails(keys []interface{}, keyField string, details []map[string]interface{}, errs []error) ([]interface{}, int) {
	list := []interface{}{}
	failed := 0
	for i, k := range keys {
		if i >= len(details) || details[i] == nil {
			item := map[string]interface{}{keyField: k}
			if i < len(errs) && errs[i] != nil {
				item["_detailError"] = errs[i].Error()
			}
			list = append(list, item)
			failed++
			continue
		}
		details[i][keyField] = k
		list = append(list, details[i])
	}
	return list, failed
}

// Secondary DNS
func GetSecondaryDNS(state *app.State) gin.HandlerFunc {
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
		dnsPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "secondaryDnsDomains")
		if !ok {
			return
		}
		var domains []string
		if err := client.Get(dnsPath, &domains); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		details, detailErrs := miscParallelGetStringKeys(client, domains, func(d string) string {
			return safePathFn("dedicated", "server", svc, "secondaryDnsDomains", d)
		}, 10)
		list, failed := miscMergeDetails(miscKeysAsIface(domains), "domain", details, detailErrs)
		if failed > 0 {
			state.Logger.Warn(fmt.Sprintf("服务器 %s 从DNS域名详情有 %d/%d 项拉取失败", svc, failed, len(domains)), "server_control")
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "domains": list, "partial": failed > 0, "failedCount": failed})
	}
}

func AddSecondaryDNS(state *app.State) gin.HandlerFunc {
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
			Domain string `json:"domain"`
		}
		_ = c.ShouldBindJSON(&body)
		body.Domain = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(body.Domain), "."))
		if body.Domain == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少domain参数", "code": "E5BE84C8C"})
			return
		}
		if !strings.Contains(body.Domain, ".") || len(body.Domain) > 255 || strings.ContainsAny(body.Domain, "/?#%& ") {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "域名格式不正确", "code": "E_INVALID_DOMAIN"})
			return
		}
		dnsPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "secondaryDnsDomains")
		if !ok {
			return
		}
		if err := client.Post(dnsPath, map[string]interface{}{"domain": body.Domain}, nil); err != nil {
			// 已添加过的域名(already exists):幂等成功
			if strings.Contains(strings.ToLower(err.Error()), "already") {
				c.JSON(http.StatusOK, gin.H{"success": true, "message": "该域名已在二级 DNS 列表中,无需重复添加", "code": "E4D9E7F86"})
				return
			}
			respondOVHError(c, err)
			return
		}
		state.Logger.Info("添加从DNS域名 "+body.Domain+" 成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "从DNS域名已添加", "code": "E10077A28"})
	}
}

func DeleteSecondaryDNS(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		domain, ok := requireSafeSingleSegment(c, "domain", "域名", "E_INVALID_DOMAIN")
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		delPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "secondaryDnsDomains", domain)
		if !ok {
			return
		}
		if err := client.Delete(delPath, nil); err != nil {
			if ovhIsNotFound(err) {
				c.JSON(http.StatusOK, gin.H{"success": true, "message": "该域名不在二级 DNS 列表中,无需删除", "code": "E1C4B0FD2"})
				return
			}
			respondOVHError(c, err)
			return
		}
		state.Logger.Info("删除从DNS域名 "+domain+" 成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "从DNS域名已删除", "code": "E3163802E"})
	}
}

// Virtual MAC
func GetVirtualMACList(state *app.State) gin.HandlerFunc {
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
		vmacPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "virtualMac")
		if !ok {
			return
		}
		var macs []string
		if err := client.Get(vmacPath, &macs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		details, detailErrs := miscParallelGetStringKeys(client, macs, func(m string) string {
			return safePathFn("dedicated", "server", svc, "virtualMac", m)
		}, 10)
		list, failed := miscMergeDetails(miscKeysAsIface(macs), "macAddress", details, detailErrs)
		if failed > 0 {
			state.Logger.Warn(fmt.Sprintf("服务器 %s 虚拟MAC详情有 %d/%d 项拉取失败", svc, failed, len(macs)), "server_control")
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "virtualMacs": list, "partial": failed > 0, "failedCount": failed})
	}
}

func CreateVirtualMAC(state *app.State) gin.HandlerFunc {
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
			IPAddress          string `json:"ipAddress"`
			Type               string `json:"type"`
			VirtualMachineName string `json:"virtualMachineName"`
		}
		_ = c.ShouldBindJSON(&body)
		// schema 里 ipAddress / type / virtualMachineName 三个 body 参数都是必填,
		// 少了名字只会换回 OVH 的英文参数错误,不如本地先说清楚缺哪个
		if body.IPAddress == "" || body.Type == "" || body.VirtualMachineName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少必需参数: ipAddress、type、virtualMachineName 都必须提供", "code": "EF8671908"})
			return
		}
		// dedicated.server.VmacTypeEnum 只有这两个取值
		if body.Type != "ovh" && body.Type != "vmware" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "无效的虚拟MAC类型: " + body.Type + " (只支持 ovh / vmware)"})
			return
		}
		vmacPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "virtualMac")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Post(vmacPath, map[string]interface{}{
			"ipAddress":          body.IPAddress,
			"type":               body.Type,
			"virtualMachineName": body.VirtualMachineName,
		}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("创建虚拟MAC成功: "+body.IPAddress, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "虚拟MAC已创建", "code": "EE9B16619", "result": result})
	}
}

// Virtual Network Interface
func GetVirtualNetworkInterfaces(state *app.State) gin.HandlerFunc {
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
		vniPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "virtualNetworkInterface")
		if !ok {
			return
		}
		var uuids []string
		if err := client.Get(vniPath, &uuids); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		details, detailErrs := miscParallelGetStringKeys(client, uuids, func(u string) string {
			return safePathFn("dedicated", "server", svc, "virtualNetworkInterface", u)
		}, 10)
		list, failed := miscMergeDetails(miscKeysAsIface(uuids), "uuid", details, detailErrs)
		if failed > 0 {
			state.Logger.Warn(fmt.Sprintf("服务器 %s 虚拟网络接口详情有 %d/%d 项拉取失败", svc, failed, len(uuids)), "server_control")
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "interfaces": list, "partial": failed > 0, "failedCount": failed})
	}
}

// (Enable/DisableVirtualNetworkInterface 已删:它们打的
//  /dedicated/server/{sn}/virtualNetworkInterface/{uuid}/enable|disable 三区 DEPRECATED,
//  按约定废弃端点不再调用。schema 给的 replacement(/ola/aggregation)语义是
//  "把多个接口聚合成一个",不等价于单接口启停,不照搬。虚拟网卡启停请到 OVH 控制台。)

// vRack
func GetVRackList(state *app.State) gin.HandlerFunc {
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
		vrPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "vrack")
		if !ok {
			return
		}
		var vracks []string
		if err := client.Get(vrPath, &vracks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		details, detailErrs := miscParallelGetStringKeys(client, vracks, func(v string) string {
			return safePathFn("dedicated", "server", svc, "vrack", v)
		}, 10)
		list, failed := miscMergeDetails(miscKeysAsIface(vracks), "vrackName", details, detailErrs)
		if failed > 0 {
			state.Logger.Warn(fmt.Sprintf("服务器 %s vRack 详情有 %d/%d 项拉取失败", svc, failed, len(vracks)), "server_control")
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "vracks": list, "partial": failed > 0, "failedCount": failed})
	}
}

func RemoveFromVRack(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		vrack, ok := requireSafeSingleSegment(c, "vrack", "vRack", "INVALID_VRACK")
		if !ok {
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		delPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "vrack", vrack)
		if !ok {
			return
		}
		if err := client.Delete(delPath, nil); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("从vRack "+vrack+" 移除服务器成功", "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "服务器已从vRack移除", "code": "EAC9B25F0"})
	}
}

// Orderable
func GetOrderableBandwidth(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "orderable", "bandwidth")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "orderable": d})
	}
}

func GetOrderableTraffic(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "orderable", "traffic")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "orderable": d})
	}
}

func GetOrderableIP(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "orderable", "ip")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "orderable": d})
	}
}

// Options
func GetServerOptions(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "option")
		if !ok {
			return
		}
		var opts []string
		if err := client.Get(p, &opts); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		details, detailErrs := miscParallelGetStringKeys(client, opts, func(o string) string {
			return safePathFn("dedicated", "server", svc, "option", o)
		}, 10)
		list, failed := miscMergeDetails(miscKeysAsIface(opts), "option", details, detailErrs)
		if failed > 0 {
			state.Logger.Warn(fmt.Sprintf("服务器 %s 选项详情有 %d/%d 项拉取失败", svc, failed, len(opts)), "server_control")
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "options": list, "partial": failed > 0, "failedCount": failed})
	}
}

// IP specs
func GetIPSpecs(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "specifications", "ip")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "ipSpecs": d})
	}
}

func GetIPCanBeMovedTo(state *app.State) gin.HandlerFunc {
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
		// schema: ip 是 query 必填(类型 ipBlock,可以带掩码)
		ip := strings.TrimSpace(c.Query("ip"))
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少ip参数", "code": "EB7965AF4"})
			return
		}
		if _, ok := validateIPOrCIDR(ip); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ip 参数格式不合法", "code": "INVALID_IP"})
			return
		}
		checkPath, ok := ovhPathQueryOrAbort(c, url.Values{"ip": {ip}}, "dedicated", "server", svc, "ipCanBeMovedTo")
		if !ok {
			return
		}
		if err := client.Get(checkPath, nil); err != nil {
			state.Logger.Warn("检查 IP "+ip+" 能否迁移到服务器 "+svc+" 失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err), "ip": ip})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "canBeMoved": true, "ip": ip})
	}
}

func GetIPCountryAvailable(state *app.State) gin.HandlerFunc {
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
		p, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "ipCountryAvailable")
		if !ok {
			return
		}
		var d interface{}
		if err := client.Get(p, &d); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "countries": d})
	}
}

func MoveIP(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		svc, ok := requireServiceName(c)
		if !ok {
			return
		}
		// /ipMove 的语义是"把这个 IP 搬到 URL 里的这台服务器",目标就是 svc 本身。
		// 防呆护栏:若旧调用方误把源实例放在路由 :service_name、目标实例放在请求体 to 中,
		// 必须立即 400 拦截,绝不把故障转移 IP 反向迁回源机器！
		var body struct {
			IP string `json:"ip"`
			To string `json:"to"`
		}
		_ = c.ShouldBindJSON(&body)
		body.IP = strings.TrimSpace(body.IP)
		body.To = strings.TrimSpace(body.To)
		if body.IP == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少ip参数", "code": "EB7965AF4"})
			return
		}
		if _, ok := validateIPOrCIDR(body.IP); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "ip 参数格式不合法", "code": "INVALID_IP"})
			return
		}
		if body.To != "" && !strings.EqualFold(body.To, svc) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "路由服务名与请求体目标服务器(to)冲突：POST /server-control/{service_name}/ip/move 中的 {service_name} 必须为迁入目标服务器",
				"code":    "MOVE_IP_TARGET_MISMATCH",
			})
			return
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		movePath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "ipMove")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Post(movePath, map[string]interface{}{
			"ip": body.IP,
		}, &result); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		state.Logger.Info("IP迁移任务已创建: "+body.IP+" -> "+svc, "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "IP迁移任务已创建", "code": "E26999A49", "result": result})
	}
}

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
