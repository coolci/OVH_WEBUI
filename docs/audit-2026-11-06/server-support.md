# 已购服务器、VPS、支持与账户账务审计

> **2026-11-06修复更新：本报告确认项已完成代码修复与对应离线回归。** 方案见[FIXES](FIXES.md)，当前结果见[TESTS](TESTS.md)。以下是发现时记录，“待修复”、错误证明与旧日志仅描述修复前基线。TL-01历史凭据外部轮换未执行。

## 发现时记录（历史）

审计日期：2026-11-06。基线为当前工作树（全项目索引记录 commit `891e20bb055abd0ddfe69903c96a833b884727db`）。本文件只报告问题与方案，没有修改业务源码，没有向真实 OVH 提交工单、付款、撤单、重装、快照或电源操作。

本域确认 **5 项缺陷：1 项 P1，4 项 P2**。最高优先级是“保留磁盘数据”配置未进入重装请求。恢复审计后补读全部本域业务 UI，新增 SS-05 反向 DNS 目标 IP 丢失的隔离复现。另有待核实事项独立列出，不计入确认缺陷。证据是实际源码执行后的 mock 请求/响应；没有声称真实服务器测试通过。

## 确认缺陷

### SS-04 · P1 · 只勾选保留磁盘组时，重装请求丢失数据保留配置

- 入口：`src/components/server-control/reinstall/StoragePartitionSection.tsx:309` 为每个磁盘组提供“保留数据”复选框；独立于自定义分区、软 RAID、硬 RAID。`src/components/server-control/ReinstallDialog.tsx:236` 在高级存储模式把 `keepDiskGroups` 传给重装 hook。
- 根因：`src/hooks/ovh/server-control/use-server-install.ts:67` 的 `useCustom` 只检查软 RAID、硬 RAID、自定义分区，未检查 `keepDiskGroups`。`erase:false` 只在 `if (useCustom)` 内的第 143 行追加。
- 触发：两组磁盘，切换到高级存储，只勾选第二组保留数据，不添加分区、不启用 RAID，然后确认重装。
- 实际结果：hook 收到 `keepDiskGroups:[2]`，发送的 body 没有 `storageConfig`，也没有任何 `erase:false`。后端 `backend/internal/handlers/server_control_basic.go:745` 判定没有自定义存储，在第 946 行走默认分区，随后第 957 行提交 OVH `/dedicated/server/{svc}/reinstall`。
- 影响：用户明确勾选的数据保留约束完全失效；实际擦除范围由 OVH 默认重装决定，存在非安装盘数据被擦除风险。**已证明请求丢失，不是通过实盘擦除来验证风险。**
- 复现：`server-support-proof.mjs` 的 `SS-04` 执行真实 hook 的 `mutationFn`，断言捕获 body 不含存储配置，通过。mock 输入保留第 2 组，输出为默认重装请求。
- 最优修复方向：把存储模式和保留要求表示为显式请求契约；保留项不能因“没有 RAID/分区”而被省略。优先让后端拒绝无法明确确定安装组与保留组的组合；如果支持默认分区加保留组，应明确映射安装组并生成保留组 `erase:false`，校验组 ID 唯一、安装组不能同时保留。不要仅在 UI 隐藏复选框。
- 修复测试：只保留组、分区+保留组、硬 RAID+保留组、同组同时安装与保留、默认组 ID、重复组、全部组保留；断言最终 OVH 请求，而不只断言 React state。

### SS-01 · P2 · VPS 快照回滚确认后仍必定被后端拒绝

- 入口：`src/components/vps-control/VpsSnapshotPane.tsx:62` 检查输入的服务名，第 68 行执行 `revert.mutateAsync()`。
- 根因：`src/hooks/ovh/use-vps-control.ts:347` POST `/vps-control/{svc}/snapshot/revert` 时不发送 body。`backend/internal/handlers/vps_control_snapshot.go:151` 明确拒绝未携带 `confirm:true` 的请求。
- 触发：已有快照，点击回滚，输入正确 VPS 服务名并确认。
- 影响：界面回滚功能不可用；不会进入 OVH 回滚调用。
- 复现：真实 hook 捕获 body=`undefined`；真实 Gin handler 加隔离账户 transport 收到相同请求，响应 **HTTP 400 / `EFD5D37BB`**。两个测试均通过，且没有真实回滚。
- 最优修复方向：UI 完成服务名确认后，hook 显式提交 `{confirm:true}`，保留后端校验。增加前端 payload 到 handler 的契约测试；缺少确认仍应 400，正确确认只产生一次 mock 上游 POST。

### SS-02 · P2 · 故障、暂停和过期的独服被筛掉，维护入口消失

- 根因：`src/hooks/ovh/server-control/use-server-core.ts:18` 的列表过滤，明确剔除 billing `expired/suspended`（第 22 行）、server `error/suspended`（第 23 行），并只保留 `ok/active`（第 24 行）。
- 调用链：后端 `ListMyServers` 返回机器及状态 → `useOwnedServers` → `useServerLotterySummary` → `src/pages/ServerControlPage.tsx:55` → 第 127 行设备选择器、第 163 行维护页。
- 触发：OVH 仍返回机器，但机器进入 error/suspended，或服务状态进入 suspended/expired。
- 影响：最需要救援、检查账务和续费的机器无法在控制页选择；页面甚至可能显示“暂无服务器”。后端详情请求失败的机器反而通过 `if(s.error)return true` 保留，正常返回故障状态的机器却消失。
- 复现：API mock 返回 5 台（正常、故障、账务暂停、账务过期、installing）；执行真实 queryFn 后只有正常 1 台保留。
- 最优修复方向：列表保留 API 返回的全部已有服务，状态决定提醒和可执行动作，不决定整台机器是否可见。对无法执行的操作禁用并解释原因。
- 修复测试：各种服务状态仍可选择；正常、异常详情占位均保留；不存在的服务不得被前端凭空创建。

### SS-03 · P2 · 账户记录详情全部失败，被显示为“暂无记录”

- 后端根因：`backend/internal/handlers/account.go:131` 的 `collectDetails` 丢弃失败记录，第 144 行只写 `X-Partial-Failures`；邮件第 413 行、退款第 353 行、订单第 741 行随后仍返回 HTTP 200。订单虽另有 `failed`，前端第 117 行只提取 `orders`。
- 前端：`src/hooks/ovh/use-account.ts` 的邮件/退款只返回 body，不读取 partial header；订单同样丢失 `total/failed/warning`。`src/pages/AccountPage.tsx:160`、第 235 行、第 338 行把空数组显示为“暂无邮件/订单记录/退款记录”，缺少相应失败分支。
- 触发：列表 ID 查询成功，但详情因权限、网络错误或限流全部失败；也包括列表请求本身失败后首次无缓存的情况。
- 影响：已有财务/邮件记录被误显示为不存在，用户无法辨别权限和网络问题；部分失败时同样没有可见缺漏提示。
- 复现：两个已有邮件 ID 的 detail 均 mock 403；真实 handler 返回 **HTTP 200 `[]`，`X-Partial-Failures:2`**。订单真实 hook 接收 `{orders:[],total:2,failed:2}` 后只返回 `[]`。均已通过离线测试。
- 最优修复方向：全部详情失败时返回可重试错误；部分成功保留数据并显式返回 partial/failed。hook 保留元数据；UI 区分初次错误、旧缓存刷新失败、空记录、部分记录。邮件/退款如需兼容裸数组，可先由 hook读取 header，但全部失败仍应失败响应。

### SS-05 · P2 · 从第二个 IP 打开反向 DNS，提交目标被替换成第一个 IP

- 入口：`src/components/server-control/OverviewTab.tsx:181` 点击每个 IP 旁的编辑按钮，将该 IP 存入 selectedIp；第 235–238 行把它作为 defaultIp 传给始终挂载的 ReverseDnsDialog。
- 根因：`src/components/server-control/ReverseDnsDialog.tsx:41` 只在组件首次挂载时读取 defaultIp，没有在打开或目标变化时同步；第 139 行“设置反向 DNS”按钮又无条件设置为 `serverIps[0]?.ip`。编辑表单没有 IP 选择器，无法从该入口选择尚未配置 PTR 的第二个 IP。
- 触发：同一独服有两个 IP，第二个 IP 尚无 PTR；用户点击第二个 IP 的编辑入口，再点“设置反向 DNS”，输入域名并保存。
- 实际请求：选择 `192.0.2.2`，真实组件的保存逻辑经真实 `useSetServerReverseDns` 发出的 POST `/server-control/ns-fixture/reverse` 却为 `{ip:"192.0.2.1",reverse:"secondary.example.test"}`。
- 后端影响：`src/hooks/ovh/server-control/use-server-network.ts:192` 原样转发；`backend/internal/handlers/server_control_hardware.go:370` 只检查 IP 属于该机器，第 377 行按该 IP 写上游 `ipReverse`。第一个 IP 同样属于该机器，因此所有权校验无法纠正意图；上游可能拒绝域名校验，也可能修改了错误 IP 的 PTR。测试未访问真实 DNS。
- 复现：`server-support-proof.mjs` 的 SS-05 转译真实 TSX，使用隔离 React 状态与渲染节点执行“打开 → 设置 → 输入 → 保存”，HTTP 为 mock；观察到目标 IP 确实变成第一项。
- 最优修复方向：按 open/serviceName/defaultIp 初始化编辑目标和当前反向值；创建入口优先使用已选择 IP，并为多个地址提供明确选择器。确认与提交都展示同一个目标 IP，保留后端归属校验。
- 修复测试：第二个无 PTR 的 IPv4、IPv6、已有 PTR 编辑、关闭后改选另一个 IP、切换服务器、IP 清单失败；断言最终 HTTP body 与用户所选地址一致。

## 业务链路提取

以下是本域业务边界。精确函数位置、全部 HTTP 路由和调用点分别参见同目录 `FUNCTION-INDEX.md`、`ENDPOINTS.md`、`API-CALL-INDEX.md`；这些是全文件自动索引，不能等同于逐函数动态测试。这里补充入口、载荷、校验、上游调用与状态处理。

共同规则：独服和 VPS 控制路由均经 `ValidateServiceName`；绝大部分 handler 用 `ovhClientFor` 按显式 `?account=` 取得隔离 SDK 客户端，未指定账户才走默认账户，无法取得客户端返回 412。前端 `http.ts` 对独服/账户/工单注入独服活动账户，对 VPS 注入 VPS 活动账户。列表 query key 含账户；多数详情按服务名建 key。OVH 写操作一般不做客户端自动重试，错误通过 handler 响应和 toast/内联提示呈现。

### 独服控制

| 独立业务 | 入口与载荷 | 后端、OVH 调用、结果与失败处理 |
|---|---|---|
| 机器清单/选择 | ServerControlPage → useOwnedServers，GET list | ListMyServers：`/dedicated/server` 后隔离并发读取机器、serviceInfos、中奖信息；详情失败保留占位。前端状态过滤有 SS-02 |
| 本地别名查询/保存/删除 | 选择器重命名；alias字符串 | server_aliases → ServerService → AliasRepository，按账户保存；空 alias 删除，不调用 OVH |
| 硬件与中奖对比 | OverviewTab、硬件摘要，GET hardware | hardware + services billing plan/options；比较 CPU 型号、内存、磁盘，缓存订购配置/1小时中奖摘要；失败不会阻止基本机器列表 |
| 网络规格与 IP 规格 | NetworkSpecsDialog，GET network-specs/ip-specs | specifications/network、specifications/ip；返回上游带宽、路由、流量、OLA、vRack等字段 |
| IP 清单 | Overview/网络 hook | dedicated ips → `/ip/{block}`详情；失败保留地址并用默认类型/服务路由占位 |
| PTR 查询 | ReverseDnsDialog GET reverse | 先服务器 IP块，再各 reverse集合及详情；部分失败返回partial，全部失败返回错误 |
| PTR 写入/删除 | `{ip,reverse}`或path IP | 先 `findIPBlockForServer` 校验IP属于该机器块；POST/DELETE `/ip/{block}/reverse`；成功失效PTR与IP缓存。UI新增入口目标丢失见SS-05 |
| 服务信息/续费状态 | 摘要胶囊、RenewalDialog | serviceInfos + `/services/{id}` billing/lifecycle；解析自动续费、周期、终止动作；读不到终止状态标unknown |
| 自动/手动续费 | `{mode,period}` | 检查非负周期、forced锁定；`putServiceRenew`兼容新旧计费路径；delete模式被拒绝，避免混用立即终止；成功刷新serviceInfo/list |
| 到期终止/撤销终止 | `{policy}` | termination_policy 校验3个枚举，PUT `/services/{id}` terminationPolicy；成功刷新serviceInfo |
| 合同期查询 | EngagementDialog GET engagement/available/request | 先serviceId，再 `/services/{id}/billing/engagement*`；当前期/请求404表示不存在；价格列表错误保持失败 |
| 建立/撤销合同期请求 | `{pricingMode}` / DELETE request | pricingMode必填；POST/DELETE engagement/request；返回订单链接，可打开付款；刷新当前合同和请求缓存 |
| 更改合同期结束策略 | `{strategy,confirm?}` | 枚举白名单；CANCEL_SERVICE强制确认；PUT endRule；共享Dialog明确二次确认后发送confirm |
| 重启 | PowerTab确认 → POST reboot | dedicated reboot；已有重启请求按幂等成功返回；任务ID与审计日志；失败不声称已重启 |
| 启动项读取/设置 | BootModeDialog；`{bootId}`或path boot_id | GET机器当前bootId、boot集合和详情；失败占位。单个PUT handler只修改下次启动项；UI在二次确认后依次setBoot和reboot，后者失败单独警告启动项已改 |
| 监控开关 | `{monitoring:boolean}` | 兼容enabled/monitoring指针，缺字段拒绝，PUT机器；成功刷新monitoring |
| 救援状态 | RescueDialog GET rescue | GET机器和bootType=rescue启动项；返回配置状态而非证明OS已进入救援 |
| 进入救援 | `{email,sshKey,bootId,expectedBoot,confirm}` | 校验明确确认、邮箱或默认邮箱、当前boot匹配、boot属于此机器；PUT启动配置后POST reboot；第二步失败清晰说明配置已改变 |
| 退出救援 | `{expectedBoot,confirm:true}` | 校验当前启动项，找harddisk boot，PUT后reboot；已有重启幂等；成功刷新救援与boot缓存 |
| OS模板查询 | ReinstallDialog搜索与选中 | compatibleTemplates.ovh → installationTemplate详情；转换文件系统/分区能力与定制问题；前端localStorage缓存1小时策略，详见待核实项 |
| 磁盘、RAID配置、内置分区方案 | 重装高级存储 | specifications/hardware、hardwareRaidProfile、partitionScheme+partition详情；模板/方案路径转义；失败的方案详情以空分区占位 |
| 默认/内置方案重装 | `{templateName,customHostname,customizations,partitionSchemeName}` | InstallOS按服务TryLock；模板必填；映射operatingSystem/customizations/storage；POST dedicated reinstall，失败502 |
| 自定义分区/软硬 RAID/保留组 | hook把UI结构转storageConfig | normalizeStorageConfig白名单字段、RAID/FS合法性、大小非负、单个填充、erase与配置冲突校验；SS-04在hook转换之前丢失保留项 |
| Proxmox9 ZFS预设 | useProxmox9Zfs、zfsRaidLevel、zfsVzSize | 后端取硬件首磁盘组计算容量，生成boot/swap/root/vz布局；与其他存储配置同时提交时给warning；磁盘不足/容量超限拒绝 |
| 安装进度 | InstallProgressDialog，5秒轮询 | install/status；具体无任务或确认机器存在后404转无安装；统计done/error/expired/stopping/unknown，翻译步骤 |
| 最近任务 | TasksDialog GET tasks | 按taskId排序取10条，并发详情；失败保留unknown/error占位，避免任务消失 |
| 干预预约查询/提交 | TimeslotsDialog；日期范围、wantedBeginingDate、hasPerformedBackup | date/ISO规范化，availableTimeslots；无需预约独立返回；POST schedule验证ISO和备份回答必须存在，false允许但记录警告 |
| 历史/计划干预及详情 | MaintenanceTab | intervention/plannedIntervention集合与详情；部分失败标partial、全部失败错误；单详情直通 |
| 硬盘/内存/散热更换申请 | HardwareReplaceDialog；componentType、comment、disks/inverse、slots/details | 组件白名单；磁盘序列必填兼容旧别名；POST support/replace对应类型，已有工单返回isPending；返回工单号/notice，刷新interventions |
| NIC与流量/网络统计 | MRTG图、network interfaces | 校验period/type枚举；每NIC MRTG并发读取，错误附在对应卡；无网卡与读取失败区分；legacy统计接口共用NIC来源 |
| OLA聚合/分组 | name + 至少2个virtualNetworkInterfaces | aggregation/group映射同一OVH aggregation；reset/ungroup要求UUID并映射reset；返回任务，需等待上游完成 |
| IPMI支持类型/控制台 | IpmiDialog，type/sshKey query | 查supportedFeatures与activated，选优先支持类型；POST access后最多10次2秒查询task，最后GET access值；错误/超时不返回空链接 |
| 突发带宽查询/变更 | AdvancedTab；status | GET/PUT burst，status白名单；不支持单独标识；成功刷新burst |
| 硬件防火墙查询/开关 | AdvancedTab；enabled布尔 | features/firewall；缺enabled拒绝；不支持与普通失败区分；成功刷新firewall |
| BackupFTP状态/激活/删除 | AdvancedTab | EU/CA接口，US门控；feature不存在与服务不属于账户区别；变更后刷新backupFtp |
| BackupFTP ACL读/增/删 | ipBlock、ftp/nfs/cifs | 详情并发、全部失败报错；CIDR PathEscape；删除支持query/body/旧path；变更刷新FTP复合缓存 |
| BackupFTP可授权块/重置密码 | GET authorizable-blocks / POST password | 直通对应feature端点；未激活与不可用区分，重置触发上游邮件 |
| BackupCloud状态/开通/停用 | cloudProjectId/projectDescription | GET/POST/DELETE features/backupCloud；不存在时判断机器；失败保留原因，变更刷新缓存 |
| BackupCloud报价/密码 | offer-details / password | backupCloudOfferDetails、features/backupCloud/password；明确未激活/不支持 |
| 二级DNS查询/增加/删除 | domain | 小写去末尾点，基础域名校验；secondaryDnsDomains；重复添加/不存在删除有幂等处理；刷新列表 |
| 虚拟MAC查询/创建 | ipAddress,type,virtualMachineName | 必填/type=ovh或vmware；virtualMac集合及详情；查询partial提示；创建返回result |
| 虚拟网卡/vRack查询、移除vRack | UUID/name | 各集合并发详情，失败保留键值占位；DELETE服务器vrack对应项 |
| 可订购带宽/流量/IP | GET orderable三类 | 分别调用对应orderable；前端聚合hook把失败项转null，不能认为已经证明不可订购 |
| IP迁移检查/国家/执行 | ip | ipCanBeMovedTo query转义、ipCountryAvailable、POST ipMove；必填校验；返回任务 |
| 进行中任务/兼容Windows及SQL版本 | GET ongoing/compliant* | 原样读取对应dedicated端点 |
| 立即终止与邮件token确认 | POST terminate / token、reason、futureUse、commentary | 确认token必填、原因和后续用途白名单；与到期终止不同；当前主要UI走到期策略 |
| SPLA许可查询/登记 | SplaDialog；type、serialNumber | type=os/sqlstd/sqlweb；读取partial；POST spla返回ID；不得把partial记录当证明没有许可 |
| BIOS/SGX只读 | BiosDialog | GET biosSettings及sgx；404不可用，其他失败502；hook对SGX失败降级null |
| 缓解状态查询 | 独服hook/API（当前独服组件未发现调用入口） | IPv4块mitigation列表及详情，失败状态unknown；IPv6只说明此接口不适用；独服启停兼容路由固定410 |
| 订单映射扫描/只读缓存 | GET order-mapping，forceRefresh/limit；当前src未暴露同步按钮 | accountID隔离缓存10分钟，降级结果2分钟；专服创建日期±15天定窗，EU/CA默认50最多200订单，US固定最近100再本地过滤；排除取消订单、按明细domain匹配，所有目标找到则早停；日期/ID确定最新，返回degraded/truncated/hint。撤单有独立回源路径，不依赖前端存在同步入口 |
| 撤单资格读取 | RetractionDialog | US不支持；按账户+服务缓存订单映射5分钟，扫描21天订单及detail.domain；核对OVH retractionDate，返回eligible/原因/截止时间 |
| 撤单提交 | `{orderId,reason,comment,confirm:true}` | 强制确认，原因白名单；重新查订单期限与服务归属，POST `/me/order/{id}/retraction`；影响整张订单，成功清除映射并刷新列表 |

### VPS 控制

| 独立业务 | 入口、校验与上游调用 | 返回/刷新/失败 |
|---|---|---|
| 清单/详情 | VpsControlPage、useOwnedVps；`/vps`及`/vps/{svc}`、serviceInfos | 并发10；错误机器保留；账务未知用null；列表key含VPS账户 |
| 服务信息与续费 | serviceInfos + billing/lifecycle；mode/period校验，forced禁止 | 共用计费续费适配；成功更新serviceInfo；到期终止走termination-policy |
| IP/PTR/数据中心 | ips→详情、PUT ips/{ip} reverse、GET datacenter | PTR规范末尾点，成功失效IP缓存；详情失败保留地址 |
| 启动/关机/重启 | VpsDetail确认→start/stop/reboot | 返回任务；start/stop任务冲突409，重复重启幂等；start/stop失效list/info |
| noVNC控制台 | POST console→OVH getConsoleUrl | 返回一次性URL，非GET副作用query；前端打开窗口 |
| 旧端口状态/重置密码 | status/password兼容路由 | 状态返回removed，密码返回410；当前hooks已不暴露旧操作 |
| 当前系统/可重建镜像 | images/current、images/available及详情 | 模板按账户+服务缓存10分钟；部分失败不缓存；全失败500，前端PartialNotice |
| 重建/重装兼容入口 | VpsReinstallDialog；templateId/imageId、单个sshKey名称、doNotSendPassword | 统一POST rebuild，imageId优先；拒绝多个key/公钥正文；服务锁；返回task并刷新tasks |
| 最近任务/单任务进度 | tasks按ID取最近10；单详情可轮询 | 详情失败被跳过，尚无与独服相同的失败占位；记录为待扩展故障用例 |
| 快照查询 | GET snapshot | 404→snapshot:null，其余错误不隐藏；UI区分空与失败 |
| 创建快照 | description→createSnapshot | 返回异步task，立即失效snapshot；task冲突409、已存在400；完成后的再次刷新见待核实项 |
| 修改快照描述 | PUT snapshot `{description}` | 只发送可写字段，404提示无快照；刷新snapshot |
| 回滚快照 | UI输入服务名→hook POST revert | 后端要求confirm，SS-01导致当前UI被拒；正确请求返回task |
| 删除快照 | DELETE snapshot | 不存在幂等成功，返回task；刷新snapshot |
| 联系人变更 | admin/tech/billing映射contact* | 至少一个，US提前拒绝；changeContact返回taskIDs |
| 合同期/到期策略 | 复用EngagementDialog及VPS hooks | serviceIDForVps→services billing；CANCEL_SERVICE确认对象在运行时原样发送；不是缺失confirm问题 |
| 立即终止及确认 | VpsDetail维护tab的终止按钮；terminate / token、reason、commentary | token必填，OVH返回字符串不是邮件token；UI有申请与输入邮件token确认两种入口（hook注释称无UI入口已过时） |
| 二级DNS增删查 | domain、可选IPv4 | 域名基础校验；非IPv4拒绝；列表、重复添加、404删除处理 |
| 附加选项/自动备份 | option集合详情、automatedBackup | US特定选项标管理接口不可用；取消选项兼容接口410；自动备份404为无备份 |
| DDoS状态与开关 | VpsMitigationPane→ips→IPblock→mitigation | IPv4校验；转场5秒轮询；仍暴露POST/DELETE。公开schema目前仍存在这些方法，不能按独服410推断上游已删除 |
| 撤单/本地别名 | 复用独服组件，prefix切换，alias显式VPS账户 | 撤单仍按真实订单及serviceName核验；别名只保存本地 |

### 工单、在线客服与账户账务

| 独立业务 | 请求/校验/OVH | 返回、缓存与失败 |
|---|---|---|
| 工单筛选搜索列表 | status/archived/q/page/pageSize；page正整数、pageSize≤50；数字或CS前缀→ticketNumber，其余→subject | `/support/tickets` IDs后8路详情；部分失败标incomplete、全失败错误；前端30秒轮询，当前UI固定第一页50条 |
| 工单详情/消息 | ticketID严格数字格式；ticket/messages | 分账户query key，详情20秒、消息15秒轮询；聊天按时间排序 |
| 新建工单 | category/subcategory/product白名单、subject≤255/body≤10000 Unicode字符、serviceName白名单 | `/support/tickets/create`；成功失效列表、清空输入、选择新工单；失败保留内容 |
| 关联现有服务 | useAccountServices同时读取独服/VPS全量清单 | 包含故障机器；两类读取catch返回[]，未对UI保留失败标识，单独列作风险 |
| 回复 | 开放工单、非空正文，ticketID合法；POST reply | 同账户和工单草稿；成功只清除已发送的草稿版本，失效消息/列表/详情；失败保留 |
| 关闭 | canBeClosed且open，POST close | 确认后提交；成功失效相应缓存；失败不提前关闭UI状态 |
| 重新打开 | closed且非空原因，POST reopen | 原因验证与回复文本共用；成功刷新列表/详情 |
| 在线客服授权 | 同步开空白窗口→后端product dedicated/vps、GET /me、POST /auth/token | no-store，token长度/非空校验；官方聊天URL；切账户abort并关闭旧窗口；不持久化token |
| 账户资料/KYC | GET /me | 直接返回nichandle字段；配置子公司不匹配只header+日志；前端账户key隔离 |
| 邮件历史 | history IDs倒序取50，10路详情，瞬时错误重试1次/单详情20秒 | 裸数组+partial header；SS-03。当前选中邮件存在组件state，切账户风险见下 |
| 退款/账单 | date.from窗口1/3/12/36月，不够退全量；最多100详情，再按date取20 | 部分失败记header/failed；窗口拒绝降级说明；前端退款SS-03 |
| 订单列表 | limit默认30上限100，日期窗口→ID倒序取样→详情；固定真实账户 | 各订单status独立10秒/5并发；移除订单password，补manager链接，返回真实accountId；前端丢failed导致SS-03 |
| 付款方式预检 | 订单ID正整数，固定账户；GET订单/status；校验订单号、未付款、未过期、金额有限/币种、数据库尝试记录 | US读取availablePaymentMethod；EU/CA筛选VALID注册方式及有效期；仅打开窗口不付款 |
| 手动付款 | UI确认绑定账户/订单/金额/币种/方式；confirm=true；服务端重新预检并精确比较 | DB原子claim防重复；45秒独立上下文POST pay；明确4xx可清claim，不确定结果保留防重复；前端禁自动重试、onSettled刷新账户缓存 |
| SSH密钥列表 | 显式账户 GET /me/sshKey | VPS重建只传已登记名称；query key含账户 |
| 联系人请求/接受/拒绝/重发 | `/me/task/contactChange*`，token/ID校验 | US不支持；列表并发详情；本项account.go后半由平台审计员互补人工阅读 |
| 信用余额/子账户 | 集合后并发详情 | collectDetails同公共部分失败机制；非AccountPage主tab |

## 运行证据

```powershell
node --test docs/audit-2026-11-06/server-support-proof.mjs
powershell -NoProfile -ExecutionPolicy Bypass -File docs/audit-2026-11-06/proofs/run-server-support.ps1
```

结果：Node **5/5**，Go overlay **2/2**。Node transpile真实TypeScript hooks，并对SS-05运行真实TSX事件函数；替换React状态/组件、React Query与HTTP依赖，不是实浏览器端到端测试。Go使用实际handler、虚构账户、完全离线transport。断言描述当前缺陷，不能当作功能正常的回归测试。Go源码在 `proofs/server_support_test.go`，由overlay虚拟映射进handler包，业务源码目录没有留下审计测试文件。没有进行真实扣款、发工单、重装、回滚或DNS修改。

## 风险与明确未确认项

1. **模板缓存更新**：`useServerTemplates` 有 localStorage初值、gcTime=Infinity，且mount/focus/reconnect均不重取。需要用实际TanStack观察器验证开启弹窗时enabled切换的行为，再决定是否判模板永久陈旧；此处不把配置直觉当确定缺陷。
2. **选中邮件跨账户**：AccountPage EmailsTab的selected对象不绑定账户，也没有账户变化清理effect。跨浏览器标签页改变活动账户可使列表变为新账户而详情保留旧邮件。已静态确认state结构，尚未执行组件状态复现，不计入5条确认缺陷。
3. **异步快照/电源刷新**：mutation只立即invalidate；上游task完成之前可能仍读取旧状态。需要补task完成后刷新测试，尚不能以静态缺少轮询断言所有场景永不更新。
4. **工单固定50条/服务搜索**：UI明确写“50条可搜索更多”，不是漏掉了已有翻页控件。服务名搜索placeholder与后端仅subject映射不一致，需产品明确是否支持按serviceName查找；列为功能契约风险。
5. **VPS mitigation语义**：2026-11-06读取[官方EU ip.json](https://eu.api.ovh.com/1.0/ip.json)确认POST/DELETE仍为PRODUCTION；PUT permanent已废弃且说明没有运行效果。VPS“永久缓解”标签可能不符合当前产品语义，未调用真实IP验证，不作为“接口不存在”的BUG。
6. **旧领域模型未接主流程**：domain/server按交付时间计算14天、CPU核数判断中奖，但实际handler用OVH订单retractionDate和型号/硬件比较。不能把未接主流程的领域方法问题冒充线上撤单逻辑问题。
7. **鉴权/签名/账务兼容**：OVH三大区完整真实权限差异、真实订单支付结果、真实重装磁盘擦除范围均未实机验证；本报告确认的是本地代码可证明的契约和状态问题。

## 阅读覆盖说明

精确逐文件人工阅读清单在 `server-support-coverage.json`，由 `tools/server-support-coverage.cjs` 生成。本次清单100个文件：本审计员完整阅读98个，个人partial 2个，**indexed-only 0个**。`full`表示完整业务实现人工阅读（可跳过重复注释/日志及纯SVG坐标常量）；`partial`明确列本人区间。account.go本人1–428及650–EOF、平台审计员429–649；util.go本人1–140、平台审计员141–EOF；两个文件并集均为1–EOF，JSON含teamReview与精确teamRanges，没有遗漏区间。没有剩余indexed-only业务文件。

恢复后补读范围包括续费/救援/撤单/重装确认、启动项自动重启、硬件更换/预约、联系人及token、IPMI、SPLA、AdvancedTab全部10个面板、VPS详情所有操作、DNS编辑、全部安装子组件、工单创建/聊天后半、只读概览/流量/规格/任务、类型/枚举/图标/动效/转导出文件。后几类只做类型与显示转换，没有新的独立OVH写操作。MRTG的双向请求hook也纳入清单；ZFS容量逻辑位于ReinstallDialog.tsx:169–192的内联useMemo，已完整复核，不存在另一个未读的zfs-capacity文件。`src/lib/smart-storage.ts` 是独立规划算法，已由root完整阅读并记录于root-coverage，避免重复冒领个人覆盖。

补读同时确认：RenewalDialog从到期终止转自动/手动时先取消终止再更新续费；RescueDialog明确二次确认但状态请求失败时未完全锁定进入按钮；IPMI按用户选择申请会话、JNLP下载、URL链接展示；FTP关闭/ACL删除和云备份停用有确认；云备份新密码仅组件状态展示；工单消息按时间和日期展示，Ctrl/⌘+Enter受输入法与pending限制，失败保留草稿。上述链路为静态人工复核，未因此声称真实上游成功或增加无证据缺陷。
