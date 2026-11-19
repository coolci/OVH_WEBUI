package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	ovhsdk "github.com/ovh/go-ovh/ovh"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/numconv"
	"github.com/ovh-webui/server/internal/ovh"
)

// GetOSTemplates GET /api/server-control/:service_name/templates
func GetOSTemplates(state *app.State) gin.HandlerFunc {
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
		compatPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "install", "compatibleTemplates")
		if !ok {
			return
		}
		var templates map[string]interface{}
		if err := client.Get(compatPath, &templates); err != nil {
			state.Logger.Error("获取服务器 "+svc+" 系统模板失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		var allNames []string
		if ovhArr, ok := templates["ovh"].([]interface{}); ok {
			for _, t := range ovhArr {
				if s, ok := t.(string); ok {
					allNames = append(allNames, s)
				}
			}
		}
		state.Logger.Info(fmt.Sprintf("获取服务器 %s 可用系统模板成功，共 %d 个", svc, len(allNames)), "server_control")

		// 串行逐个 GET 模板详情要 10-50 秒（50-100 个模板）。
		// 这里改成 10 路并发（OVH 通常允许 10-20 RPS），返回结构完全一致，仅顺序后排
		details := make([]gin.H, len(allNames))
		sem := make(chan struct{}, 10)
		var wg sync.WaitGroup
		for i, tn := range allNames {
			wg.Add(1)
			sem <- struct{}{}
			go func(idx int, name string) {
				defer wg.Done()
				defer func() { <-sem }()
				details[idx] = fetchOSTemplateDetail(client, name)
			}(i, tn)
		}
		wg.Wait()

		// 排序：常用优先
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
		for i := 1; i < len(details); i++ {
			for j := i; j > 0 && (getPriority(details[j-1]) > getPriority(details[j]) ||
				(getPriority(details[j-1]) == getPriority(details[j]) &&
					fmt.Sprintf("%v", details[j-1]["templateName"]) > fmt.Sprintf("%v", details[j]["templateName"]))); j-- {
				details[j-1], details[j] = details[j], details[j-1]
			}
		}
		ubuntuCount := 0
		for _, d := range details {
			if strings.Contains(strings.ToLower(fmt.Sprintf("%v", d["distribution"])), "ubuntu") {
				ubuntuCount++
			}
		}
		state.Logger.Info(fmt.Sprintf("返回 %d 个模板 (包括 %d 个Ubuntu)", len(details), ubuntuCount), "server_control")
		c.JSON(http.StatusOK, gin.H{"success": true, "templates": details, "total": len(details)})
	}
}

func fetchOSTemplateDetail(baseClient *ovhsdk.Client, name string) gin.H {
	fallback := gin.H{
		"templateName": name,
		"distribution": name,
		"family":       "unknown",
		"bitFormat":    64,
	}
	client, err := ovh.IsolateClient(baseClient)
	if err != nil {
		return fallback
	}
	tplPath, tErr := buildOVHPath("dedicated", "installationTemplate", name)
	var detail map[string]interface{}
	if tErr != nil || client.Get(tplPath, &detail) != nil {
		return fallback
	}
	bf := 64
	if v, ok := numconv.ToInt64(detail["bitFormat"]); ok {
		bf = int(v)
	}
	entry := gin.H{
		"templateName": name,
		"distribution": valueOr(detail, "distribution", "N/A"),
		"family":       valueOr(detail, "family", "N/A"),
		"description":  valueOr(detail, "description", ""),
		"bitFormat":    bf,
	}
	// 官方 partitioning-ovh 文档的 OS 兼容元数据(详情拉不到的字段不编造):
	// filesystems 过滤文件系统下拉;lvmReady/noPartitioning/softRaidOnlyMirroring
	// 控制存储配置可用性;customizeQuestions 驱动 OS 特定定制表单(sshKey/语言/LACP 等)
	for _, k := range []string{"filesystems", "lvmReady", "noPartitioning", "softRaidOnlyMirroring"} {
		if v, ok := detail[k]; ok && v != nil {
			entry[k] = v
		}
	}
	// 官方 /dedicated/installationTemplate/{name} 返回 inputs (dedicated.TemplateOsInput[])
	// 转换为 customizeQuestions: name, description, type, required (mandatory), enum, default
	if inputsRaw, ok := detail["inputs"].([]interface{}); ok && len(inputsRaw) > 0 {
		if questions := mapTemplateInputsToQuestions(inputsRaw); len(questions) > 0 {
			entry["customizeQuestions"] = questions
		}
	} else if v, ok := detail["customizeQuestions"]; ok && v != nil {
		entry["customizeQuestions"] = v
	}
	return entry
}

func mapTemplateInputsToQuestions(inputsRaw []interface{}) []gin.H {
	var questions []gin.H
	for _, inRaw := range inputsRaw {
		inMap, ok := inRaw.(map[string]interface{})
		if !ok {
			continue
		}
		qName, _ := inMap["name"].(string)
		if qName == "" || qName == "hostname" {
			// hostname 在控制台重装弹窗有独立专用的输入框与格式校验，不在此重复渲染
			continue
		}
		qDesc, _ := inMap["description"].(string)
		if qDesc == "" {
			qDesc = qName
		}
		qType, _ := inMap["type"].(string)
		if qType == "" {
			qType = "string"
		}
		mandatory, _ := inMap["mandatory"].(bool)
		q := gin.H{
			"name":        qName,
			"description": qDesc,
			"type":        qType,
			"required":    mandatory,
		}
		if defVal, ok := inMap["default"].(string); ok && defVal != "" {
			q["default"] = defVal
		}
		if enumRaw, ok := inMap["enum"].([]interface{}); ok && len(enumRaw) > 0 {
			var enumList []string
			for _, ev := range enumRaw {
				if es, ok := ev.(string); ok && es != "" {
					enumList = append(enumList, es)
				}
			}
			if len(enumList) > 0 {
				q["enum"] = enumList
			}
		}
		questions = append(questions, q)
	}
	return questions
}

// InstallOS POST /api/server-control/:service_name/install
func InstallOS(state *app.State) gin.HandlerFunc {
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

		mu, ok := acquireInstallLock("srv|" + acc.ID + "|" + svc)
		if !ok {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"error":   "该服务器已有重装任务正在执行，请等待完成后再试", "code": "EF9E440A0",
			})
			return
		}
		defer mu.Unlock()
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "请求体格式错误: " + ovh.Explain(err)})
			return
		}
		templateName, _ := body["templateName"].(string)
		if templateName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "未指定系统模板", "code": "ED4AB4A93"})
			return
		}

		installParams := map[string]interface{}{
			"operatingSystem": templateName,
		}
		if customizations := extractInstallCustomizations(state, body); len(customizations) > 0 {
			installParams["customizations"] = customizations
		}

		useZFS, _ := body["useProxmox9Zfs"].(bool)
		schemeName, _ := body["partitionSchemeName"].(string)
		schemeName = strings.TrimSpace(schemeName)
		hasCustomStorage := isNonEmptyStorage(body["storageConfig"])
		if useZFS && hasCustomStorage {
			// A preset must never silently override a request to preserve data.
			groups, _ := body["storageConfig"].([]interface{})
			for _, raw := range groups {
				group, _ := raw.(map[string]interface{})
				if erase, ok := group["erase"].(bool); ok && !erase {
					c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "保留磁盘数据不能与 ZFS 预设同时启用，请明确配置安装磁盘组"})
					return
				}
			}
		}
		// 三条路径都往同一个 storage 字段写，同时给必然互相覆盖。但不能一律 400：
		// 前端 ReinstallDialog 的 "Proxmox 9 + ZFS" 开关默认就是勾上的（useProxmox9Zfs 只受模板约束），
		// 和"高级存储配置"/"内置分区方案"在 UI 上并不互斥，一律拒绝会把本来能跑通的 proxmox9 重装打成硬失败。
		// 所以只有 storageConfig 与 partitionSchemeName 这两个"用户亲手填的存储配置"同时出现时才算真歧义；
		// ZFS 预设与它们撞车时沿用历史行为（预设优先），把被忽略的那份写进日志和响应，用户看得见但装得成。
		if hasCustomStorage && schemeName != "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "自定义存储配置与内置分区方案只能选择一种", "code": "EA940C92F",
			})
			return
		}
		var installWarnings []string
		if useZFS && (hasCustomStorage || schemeName != "") {
			ignored := "自定义存储配置"
			if !hasCustomStorage {
				ignored = "内置分区方案 " + schemeName
			}
			warn := "已启用 Proxmox 9 + ZFS 预设，本次忽略同时提交的" + ignored
			installWarnings = append(installWarnings, warn)
			state.Logger.Warn(warn, "server_control")
		}

		if useZFS {
			storage, ok := buildProxmoxZFSStorage(state, client, c, svc, body)
			if !ok {
				return
			}
			installParams["storage"] = storage
		} else if hasCustomStorage {
			// 之前是把前端的 storageConfig 原样透传，字段名/类型跟 schema 对不上，
			// 硬件 RAID 重装 100% 被 OVH 拒；现在显式映射成 schema 结构
			storage, serr2 := normalizeStorageConfig(body["storageConfig"])
			if serr2 != nil {
				resp2 := gin.H{"success": false, "error": ovh.Explain(serr2)}
				// 结构化校验错误带 code + params,前端按语言插值出译文
				var se *storageError
				if errors.As(serr2, &se) {
					resp2["code"] = se.code
					resp2["params"] = se.params
				}
				c.JSON(http.StatusBadRequest, resp2)
				return
			}
			state.Logger.Info("使用自定义存储配置", "server_control")
			installParams["storage"] = storage
		} else if schemeName != "" {
			// 前端选的 OVH 预置分区方案之前被整个丢掉了；
			// schema 里它对应 storage[].partitioning.schemeName
			state.Logger.Info("使用分区方案: "+schemeName, "server_control")
			installParams["storage"] = []map[string]interface{}{
				{"partitioning": map[string]interface{}{"schemeName": schemeName}},
			}
		} else {
			state.Logger.Info("使用默认分区配置", "server_control")
		}

		state.Logger.Info("准备发送安装请求到OVH API", "server_control")
		state.Logger.Info("  - 服务器: "+svc, "server_control")
		state.Logger.Info("  - 模板: "+templateName, "server_control")

		// 之前这里手拼签名 + 本地时钟时间戳：宿主机时钟一漂就只有重装报 Invalid signature，
		// 而且 base URL 是自己映射的，未知 endpoint 会静默落到 EU。
		// go-ovh 会先跟 /auth/time 校时再签名，并且用的就是该账户自己的 endpoint
		reinstallPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "reinstall")
		if !ok {
			return
		}
		var result map[string]interface{}
		if err := client.Post(reinstallPath, installParams, &result); err != nil {
			state.Logger.Error("重装服务器 "+svc+" 系统失败: "+err.Error(), "server_control")
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "error": "OVH API错误: " + ovh.Explain(err)})
			return
		}
		state.Logger.Info("服务器 "+svc+" 系统重装请求已发送，模板: "+templateName, "server_control")
		state.Logger.Audit("reinstall_server", c.ClientIP(), svc, fmt.Sprintf("template=%s taskId=%v", templateName, result["taskId"]))
		resp := gin.H{
			"success": true,
			"message": "服务器 " + svc + " 系统重装请求已发送",
			"taskId":  result["taskId"],
		}
		if len(installWarnings) > 0 {
			resp["warnings"] = installWarnings
		}
		c.JSON(http.StatusOK, resp)
	}
}

func extractInstallCustomizations(state *app.State, body map[string]interface{}) map[string]interface{} {
	customizations := map[string]interface{}{}
	if v, ok := body["customHostname"].(string); ok && v != "" {
		// schema: 主机名在 dedicated.server.reinstall.Customizations.hostname，
		// 顶层 customHostname 是旧 /install/start 的写法，reinstall 不认这个字段
		customizations["hostname"] = v
		state.Logger.Info("设置自定义主机名: "+v, "server_control")
	}
	// 官方 api-os-installation 文档:customizations 是 OS 特定问题表
	// (sshKey / postInstallationScript / language / enableLacpBonding …)。
	// 键名与取值由 /dedicated/installationTemplate 的 customizeQuestions 定义,
	// 前端动态渲染后原样回传,这里按 schema 的值类型透传,不替 OVH 做白名单
	if cm, ok := body["customizations"].(map[string]interface{}); ok {
		for k, v := range cm {
			switch v.(type) {
			case string, bool, float64, int:
				if vs, isStr := v.(string); isStr && strings.TrimSpace(vs) == "" {
					continue
				}
				customizations[k] = v
			}
		}
	}
	return customizations
}

var translationMap = map[string]string{
	"Pre-configuring Post-installation":          "预配置安装后脚本",
	"Downloading OS image":                       "下载系统镜像",
	"Deploying OS on disks":                      "部署系统到磁盘",
	"Configuring Boot":                           "配置启动项",
	"Checking Partitioning":                      "检查分区",
	"Switching boot":                             "切换启动模式",
	"Running Last Reboot":                        "执行最后重启",
	"Waiting for services to be up":              "等待服务启动",
	"Publishing Admin password on API":           "发布管理员密码到API",
	"Checking BIOS version":                      "检查BIOS版本",
	"Running Hardware Reboot":                    "执行硬件重启",
	"Setting up hardware raid":                   "配置硬件RAID",
	"Preparing disks for new Partitioning":       "准备磁盘分区",
	"Checking hardware":                          "检查硬件",
	"Initializing hardware":                      "初始化硬件",
	"Preparing installation":                     "准备安装",
	"Partitioning disk":                          "分区磁盘",
	"Partitioning disks":                         "分区磁盘",
	"Cleaning Partitioning":                      "清理分区",
	"Processing Partitioning":                    "处理分区",
	"Applying Partitioning":                      "应用分区配置",
	"Formatting partitions":                      "格式化分区",
	"Installing system":                          "安装系统",
	"Installing system files":                    "安装系统文件",
	"Installing packages":                        "安装软件包",
	"Installing bootloader":                      "安装引导程序",
	"Installing grub":                            "安装GRUB引导",
	"Configuring system":                         "配置系统",
	"Configuring network":                        "配置网络",
	"Setting up network":                         "设置网络",
	"Setting up system":                          "设置系统",
	"Applying configuration":                     "应用配置",
	"Processing Post-installation configuration": "处理安装后配置",
	"Finalizing installation":                    "完成安装",
	"Rebooting":                                  "重启中",
	"Rebooting server":                           "重启服务器",
	"Reboot":                                     "重启",
	"First boot":                                 "首次启动",
	"Booting":                                    "启动中",
	"Starting services":                          "启动服务",
	"Starting system services":                   "启动系统服务",
	"Enabling services":                          "启用服务",
	"Installation completed":                     "安装完成",
	"Installation finished":                      "安装完成",
	"Done":                                       "完成",
	"Completed":                                  "已完成",
	"Wiping disks":                               "擦除磁盘",
	"Cleaning disks":                             "清理磁盘",
	"Creating partitions":                        "创建分区",
	"Creating filesystems":                       "创建文件系统",
	"Mounting filesystems":                       "挂载文件系统",
	"Fetching image":                             "获取镜像",
	"Extracting image":                           "解压镜像",
	"Copying files":                              "复制文件",
	"Generating configuration":                   "生成配置",
	"Writing configuration":                      "写入配置",
	"Setting hostname":                           "设置主机名",
	"Configuring timezone":                       "配置时区",
	"Configuring locale":                         "配置语言",
	"Generating SSH keys":                        "生成SSH密钥",
	"Setting root password":                      "设置root密码",
	"Managing Admin password":                    "管理管理员密码",
	"Publishing password":                        "发布密码",
	"Sending end of installation mail":           "发送安装完成邮件",
	"Sending notification":                       "发送通知",
	"Notifying completion":                       "通知完成",
	"Failed":                                     "失败",
	"Failed to download":                         "下载失败",
	"Failed to install":                          "安装失败",
	"Error":                                      "错误",
	"Partition error":                            "分区错误",
	"Boot configuration failed":                  "启动配置失败",
	"Network configuration failed":               "网络配置失败",
	"Timeout":                                    "超时",
}

// translationOrder 是 translationMap 的键按长度降序排的副本。
// map 迭代顺序随机，而 "Failed"/"Failed to download" 这类键互相包含，
// 直接 range 匹配会让同一条状态在前端轮询之间来回跳译，固定成最长匹配优先
var translationOrder = func() []string {
	keys := make([]string, 0, len(translationMap))
	for k := range translationMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	return keys
}()

func translateInstallStep(comment string) string {
	if strings.TrimSpace(comment) == "" {
		return comment
	}
	for _, eng := range translationOrder {
		if strings.EqualFold(comment, eng) {
			return translationMap[eng]
		}
	}
	commentLower := strings.ToLower(comment)
	for _, eng := range translationOrder {
		if strings.Contains(commentLower, strings.ToLower(eng)) {
			return translationMap[eng]
		}
	}
	return comment
}

// GetInstallStatus GET /api/server-control/:service_name/install/status
func GetInstallStatus(state *app.State) gin.HandlerFunc {
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
		statusPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc, "install", "status")
		if !ok {
			return
		}
		var status map[string]interface{}
		if err := client.Get(statusPath, &status); err != nil {
			// 之前拿 err.Error() 做子串匹配，"404" 三个字必然命中 go-ovh 的错误文案，
			// 服务器名写错/机器不属于当前账户也被当成"没有安装任务"。
			// 改成判 HTTP 状态码，并且 404 时多问一次服务器本体，
			// 确认机器确实存在才敢降级成"没有安装"
			var apiErr *ovhsdk.APIError
			if errors.As(err, &apiErr) {
				// 只认 OVH 明确说"这台机器没有安装任务"的这几句文案；
				// 原来的列表里还有裸的 "404"/"not found"，那种任何 404 都命中
				lower := strings.ToLower(apiErr.Message)
				for _, ind := range []string{"no installation", "no os installation", "not installing",
					"installation not found", "not being installed", "not being reinstalled",
					"being installed or reinstalled at the moment"} {
					if strings.Contains(lower, ind) {
						state.Logger.Info("服务器 "+svc+" 当前没有正在进行的安装: "+apiErr.Message, "server_control")
						c.JSON(http.StatusOK, gin.H{
							"success":         true,
							"hasInstallation": false,
							"message":         "当前没有正在进行的安装", "code": "E2BA2B9D1",
						})
						return
					}
				}
				if apiErr.Code == http.StatusNotFound {
					// 404 文案不确定时多问一次服务器本体：机器确实在，才敢说"没有安装"
					srvPath, ok := ovhPathOrAbort(c, "dedicated", "server", svc)
					if !ok {
						return
					}
					var info map[string]interface{}
					if serr := client.Get(srvPath, &info); serr != nil {
						state.Logger.Error("获取服务器 "+svc+" 安装状态失败(服务器不可访问): "+serr.Error(), "server_control")
						c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "服务器不存在或不属于当前账户", "code": "E834268F5"})
						return
					}
					state.Logger.Info("服务器 "+svc+" 当前没有正在进行的安装", "server_control")
					c.JSON(http.StatusOK, gin.H{
						"success":         true,
						"hasInstallation": false,
						"message":         "当前没有正在进行的安装", "code": "E2BA2B9D1",
					})
					return
				}
			}
			state.Logger.Error("获取服务器 "+svc+" 安装状态失败: "+err.Error(), "server_control")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": ovh.Explain(err)})
			return
		}
		elapsedTime := 0
		if v, ok := numconv.ToInt64(status["elapsedTime"]); ok {
			elapsedTime = int(v)
		}
		// schema 里 progress 可空：null 不等于"0 步全没做完"，
		// 否则进度条会一直停在 0% 且 allDone 永远为 false 而没人知道为什么
		progressRaw, hasProgress := status["progress"]
		progressUnknown := !hasProgress || progressRaw == nil
		progressArr, _ := progressRaw.([]interface{})
		totalSteps := len(progressArr)
		completed := 0
		hasError := false
		stopping := false
		formatted := []gin.H{}
		for _, sRaw := range progressArr {
			step, _ := sRaw.(map[string]interface{})
			st, _ := step["status"].(string)
			comment, _ := step["comment"].(string)
			errMsg, _ := step["error"].(string)
			// dedicated.server.InstallationProgressStatusEnum 三区一致:
			// [doing, done, error, expired, idle, pending, stopping, todo]
			// 以前只认 done/error —— expired(超时) 和 stopping(中止中) 会让进度条
			// 永远停在中间不动,用户以为还在装,一直等下去。
			if st == "done" {
				completed++
			}
			if st == "error" || st == "expired" {
				hasError = true
			}
			if st == "stopping" {
				stopping = true
			}
			formatted = append(formatted, gin.H{
				"comment":         translateInstallStep(comment),
				"commentOriginal": comment,
				"status":          st,
				"error":           errMsg,
			})
		}
		progressPercentage := 0
		if totalSteps > 0 {
			progressPercentage = completed * 100 / totalSteps
		}
		allDone := totalSteps > 0 && completed == totalSteps
		if progressUnknown {
			state.Logger.Warn("服务器 "+svc+" 安装进度为空(OVH 未返回 progress)", "server_control")
		}
		state.Logger.Info(fmt.Sprintf("获取服务器 %s 安装进度: %d%%", svc, progressPercentage), "server_control")
		c.JSON(http.StatusOK, gin.H{
			"success":         true,
			"hasInstallation": true,
			"status": gin.H{
				"elapsedTime":        elapsedTime,
				"progressPercentage": progressPercentage,
				"totalSteps":         totalSteps,
				"completedSteps":     completed,
				"hasError":           hasError,
				"stopping":           stopping,
				"allDone":            allDone,
				"progressUnknown":    progressUnknown,
				"steps":              formatted,
			},
		})
	}
}
