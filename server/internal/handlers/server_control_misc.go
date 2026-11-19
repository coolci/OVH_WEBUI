package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/ovh"
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
