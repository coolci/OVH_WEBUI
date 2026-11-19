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
	"github.com/ovh-webui/server/internal/ovh"
)

// hardwareParallelGetDetails 与 util.go 的 parallelGetDetails 行为一致,但额外把每个
// goroutine 的 error 按索引保留下来。
// 之所以不复用原版:本文件里的几个列表接口(反向 DNS / 干预记录)拉的都是需要独立授权域的
// 资源(/ip/** 与 /dedicated/server/** 是两套 access rule),用户粘贴的 consumerKey 很可能只
// 覆盖其中一个;原版"err != nil 就当这条不存在"会把 403 / 429 变成"列表是空的",用户会误以为
// 记录被删了。保留 error 才能区分"OVH 确实没有这条"和"我们没拿到"。
func hardwareParallelGetDetails(client *ovhsdk.Client, keys []interface{}, pathFn func(interface{}) string, concurrency int) ([]map[string]interface{}, []error) {
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
			// OVH 偶尔对已消失的资源回 200 + null,这里也算"没拿到",
			// 保证 results[idx]==nil 与 errs[idx]!=nil 严格同步(调用方靠这个统计失败数)
			if d == nil {
				errs[idx] = fmt.Errorf("OVH 返回了空对象: %s", p)
				return
			}
			results[idx] = d
		}(i, k)
	}
	wg.Wait()
	return results, errs
}

// hardwareFirstErr 返回一组结果里第一个非空 error(用于把"部分失败"里最有代表性的原因回给前端)
func hardwareFirstErr(errs ...[]error) error {
	for _, group := range errs {
		for _, e := range group {
			if e != nil {
				return e
			}
		}
	}
	return nil
}

// GetHardwareInfo GET /api/server-control/:service_name/hardware
func GetHardwareInfo(state *app.State) gin.HandlerFunc {
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
		acc, _ := ovhAccountFor(state, c)
		cacheScope := accountCacheFingerprint(acc)
		hwPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "specifications", "hardware")
		if !ok {
			return
		}
		var hardware map[string]interface{}
		if err := client.Get(hwPath, &hardware); err != nil {
			state.Logger.Error("获取服务器 "+svc+" 硬件信息失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		// 订购配置 vs 实际硬件 → "中奖"比对。拉不到订购配置(权限 / 老合同)不影响硬件信息本身,
		// 只是 lottery.checked=false,前端不显示中奖标识。
		spec, specErr := fetchOrderedSpec(client, cacheScope, svc)
		lottery := computeHardwareLottery(spec, hardware)
		if specErr != nil {
			lottery.Reason = ovh.Explain(specErr)
			state.Logger.Warn("获取服务器 "+svc+" 订购配置失败,跳过中奖比对: "+specErr.Error(), "server_control")
		}
		// 顺手写入结果缓存:机器列表页的徽章从这里取,列表刷新就不用再打一遍这些请求
		hwLotteryCache.Store(cacheScope+"|"+svc, hwLotteryEntry{lot: lottery, at: time.Now()})

		// 缺字段补 N/A / 0 / {} / []，
		// 否则 JSON 序列化 null 让前端 .toLowerCase / .length 崩溃
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"lottery": lottery,
			"hardware": gin.H{
				"bootMode":                valueOr(hardware, "bootMode", "N/A"),
				"coresPerProcessor":       defaultZero(hardware["coresPerProcessor"]),
				"threadsPerProcessor":     defaultZero(hardware["threadsPerProcessor"]),
				"numberOfProcessors":      defaultZero(hardware["numberOfProcessors"]),
				"processorName":           valueOr(hardware, "processorName", "N/A"),
				"processorArchitecture":   valueOr(hardware, "processorArchitecture", "N/A"),
				"memorySize":              defaultObj(hardware["memorySize"]),
				"motherboard":             valueOr(hardware, "motherboard", "N/A"),
				"formFactor":              valueOr(hardware, "formFactor", "N/A"),
				"description":             valueOr(hardware, "description", ""),
				"diskGroups":              defaultArr(hardware["diskGroups"]),
				"expansionCards":          defaultArr(hardware["expansionCards"]),
				"usbKeys":                 defaultArr(hardware["usbKeys"]),
				"defaultHardwareRaidSize": defaultObj(hardware["defaultHardwareRaidSize"]),
				"defaultHardwareRaidType": valueOr(hardware, "defaultHardwareRaidType", "N/A"),
			},
		})
	}
}

// GetNetworkSpecs GET /api/server-control/:service_name/network-specs
func GetNetworkSpecs(state *app.State) gin.HandlerFunc {
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
		netPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "specifications", "network")
		if !ok {
			return
		}
		var network map[string]interface{}
		if err := client.Get(netPath, &network); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"network": gin.H{
				"bandwidth":  network["bandwidth"],
				"connection": network["connection"],
				"ola":        network["ola"],
				"routing":    network["routing"],
				"traffic":    network["traffic"],
				"switching":  network["switching"],
				"vmac":       network["vmac"],
				"vrack":      network["vrack"],
			},
		})
	}
}

// GetHardwareRaidProfiles GET /api/server-control/:service_name/hardware-raid-profiles
func GetHardwareRaidProfiles(state *app.State) gin.HandlerFunc {
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
		profPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "install", "hardwareRaidProfile")
		if !ok {
			return
		}
		var profiles interface{}
		if err := client.Get(profPath, &profiles); err != nil {
			errMsg := strings.ToLower(err.Error())
			if strings.Contains(errMsg, "not supported") {
				c.JSON(http.StatusOK, gin.H{
					"success":   true,
					"profiles":  []interface{}{},
					"supported": false,
					"message":   "此服务器不支持硬件RAID", "code": "EBE4D06BC",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "profiles": profiles, "supported": true})
	}
}

// GetHardwareDiskInfo GET /api/server-control/:service_name/hardware-disk-info
func GetHardwareDiskInfo(state *app.State) gin.HandlerFunc {
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
		hwPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "specifications", "hardware")
		if !ok {
			return
		}
		var hardware map[string]interface{}
		if err := client.Get(hwPath, &hardware); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		diskGroups := map[string]interface{}{}
		if dgs, ok := hardware["diskGroups"].([]interface{}); ok {
			for _, dgRaw := range dgs {
				dg, ok := dgRaw.(map[string]interface{})
				if !ok {
					continue
				}
				id := 0
				if v, ok := numconv.ToInt64(dg["diskGroupId"]); ok {
					id = int(v)
				}
				numberOfDisks := 0
				if v, ok := numconv.ToInt64(dg["numberOfDisks"]); ok {
					numberOfDisks = int(v)
				}
				diskSizeValue := 0
				diskSizeUnit := "GB"
				if ds, ok := dg["diskSize"].(map[string]interface{}); ok {
					if v, ok := numconv.ToInt64(ds["value"]); ok {
						diskSizeValue = int(v)
					}
					if u, ok := ds["unit"].(string); ok {
						diskSizeUnit = u
					}
				}
				disks := []map[string]interface{}{}
				for i := 0; i < numberOfDisks; i++ {
					disks = append(disks, map[string]interface{}{
						"capacity": diskSizeValue,
						"unit":     diskSizeUnit,
						"number":   i + 1,
						"diskType": dg["diskType"],
					})
				}
				diskGroups[fmt.Sprintf("%d", id)] = map[string]interface{}{
					"id":             id,
					"diskType":       dg["diskType"],
					"description":    dg["description"],
					"raidController": dg["raidController"],
					"disks":          disks,
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "diskGroups": diskGroups, "hardware": hardware})
	}
}

// GetPartitionSchemes GET /api/server-control/:service_name/partition-schemes
func GetPartitionSchemes(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Param("service_name") != "" {
			if _, ok := requireServiceName(c); !ok {
				return
			}
		}
		client, err := ovhClientFor(state, c)
		if err != nil {
			noOVHResp(c)
			return
		}
		templateName := strings.TrimSpace(c.Query("templateName"))
		if templateName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少templateName参数", "code": "EB164A0F1"})
			return
		}
		schemesPath, ok := ovhPathOrAbort(c, "dedicated", "installationTemplate", templateName, "partitionScheme")
		if !ok {
			return
		}
		var schemes []string
		if err := client.Get(schemesPath, &schemes); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		// 双层嵌套并发：先并发拉每个 scheme 的 info + partition list，
		// 再对每个 scheme 内的 partition 并发拉详情
		type schemeResult struct {
			name        string
			info        map[string]interface{}
			parts       []string
			missingInfo bool
		}
		schemeResults := make([]schemeResult, len(schemes))
		sem := make(chan struct{}, 10)
		var wg sync.WaitGroup
		for i, schemeName := range schemes {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, sname string) {
				defer wg.Done()
				defer func() { <-sem }()
				sInfoPath, pErr := buildOVHPath("dedicated", "installationTemplate", templateName, "partitionScheme", sname)
				if pErr != nil {
					schemeResults[idx] = schemeResult{name: sname, missingInfo: true}
					return
				}
				var info map[string]interface{}
				client, err := ovh.IsolateClient(client)
				if err != nil {
					schemeResults[idx] = schemeResult{name: sname, missingInfo: true}
					return
				}
				if err := client.Get(sInfoPath, &info); err != nil {
					schemeResults[idx] = schemeResult{name: sname, missingInfo: true}
					return
				}
				var partitions []string
				if sPartPath, ppErr := buildOVHPath("dedicated", "installationTemplate", templateName, "partitionScheme", sname, "partition"); ppErr == nil {
					_ = client.Get(sPartPath, &partitions)
				}
				schemeResults[idx] = schemeResult{name: sname, info: info, parts: partitions}
			}(i, schemeName)
		}
		wg.Wait()

		details := []gin.H{}
		for _, sr := range schemeResults {
			if sr.missingInfo {
				details = append(details, gin.H{"name": sr.name, "priority": 0, "partitions": []interface{}{}})
				continue
			}
			priority := 0
			if v, ok := numconv.ToInt64(sr.info["priority"]); ok {
				priority = int(v)
			}
			// 并发拉该 scheme 的 partition 详情
			sname := sr.name
			partDetails := make([]gin.H, len(sr.parts))
			pSem := make(chan struct{}, 10)
			var pWg sync.WaitGroup
			for pi, part := range sr.parts {
				pWg.Add(1)
				pSem <- struct{}{}
				go func(pidx int, p string) {
					defer pWg.Done()
					defer func() { <-pSem }()
					partPath, pErr := buildOVHPath("dedicated", "installationTemplate", templateName, "partitionScheme", sname, "partition", p)
					if pErr != nil {
						return
					}
					var partInfo map[string]interface{}
					client, err := ovh.IsolateClient(client)
					if err != nil {
						return
					}
					if err := client.Get(partPath, &partInfo); err != nil {
						return
					}
					order := 0
					if v, ok := numconv.ToInt64(partInfo["order"]); ok {
						order = int(v)
					}
					partDetails[pidx] = gin.H{
						"mountpoint": p,
						"filesystem": valueOr(partInfo, "filesystem", ""),
						"size":       defaultZero(partInfo["size"]),
						"order":      order,
						"raid":       partInfo["raid"],
						"type":       valueOr(partInfo, "type", "primary"),
					}
				}(pi, part)
			}
			pWg.Wait()
			// 去掉拉取失败的 nil 项
			cleaned := make([]gin.H, 0, len(partDetails))
			for _, pd := range partDetails {
				if pd != nil {
					cleaned = append(cleaned, pd)
				}
			}
			// 冒泡按 order 排序
			for i := 1; i < len(cleaned); i++ {
				for j := i; j > 0; j-- {
					oi, _ := cleaned[j]["order"].(int)
					oj, _ := cleaned[j-1]["order"].(int)
					if oj > oi {
						cleaned[j-1], cleaned[j] = cleaned[j], cleaned[j-1]
					}
				}
			}
			details = append(details, gin.H{
				"name":       sr.name,
				"priority":   priority,
				"partitions": cleaned,
			})
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "schemes": details})
	}
}
