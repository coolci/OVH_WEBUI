# 前端公共链路、仪表盘与日志审计

> **2026-11-06修复更新：本报告确认项已完成代码修复与对应离线回归。** 方案见[FIXES](FIXES.md)，当前结果见[TESTS](TESTS.md)。以下是发现时记录，“待修复”、错误证明与旧日志仅描述修复前基线。TL-01历史凭据外部轮换未执行。

## 发现时记录（历史）

审计基线：`891e20bb055abd0ddfe69903c96a833b884727db`。这里记录当前可达界面的问题；未接入的旧组件不作为线上故障计算。业务源码未修改。

## 独立逻辑清单

| ID | 入口 | 前后端 / 状态流 | 判定 |
|---|---|---|---|
| UI-L01 | main.tsx / App.tsx | 旧密钥迁移 → i18n → QueryClient → AuthGate → OvhCredsGate → 路由 → ErrorBoundary | 密钥/账户验证另见 platform；已构建、浏览器执行 |
| UI-L02 | AppLayout / AppSidebar / MobileBottomNav | 13个业务页面导航、移动端菜单、快速下单弹框、路由选中 | 常规导航可用；快捷键遗漏见 UI-02 |
| UI-L03 | TopBar / StatusBar / useBackendConnection | health 探测 → 连接标识；本地日志预览 → 已读集合；默认账户显示 | health只能证明后端可达；不是OVH连通探测 |
| UI-L04 | Index / useStats / GetStats | 本地队列/目录/购买计数 → KPI/活跃队列/库存雷达 | API健康与现货标签不准确见 UI-05 |
| UI-L05 | useSystemMetrics / TelemetryCard | 2秒轮询 → 响应形状验证 → CPU/内存/磁盘显示 | 形状校验存在；失败时仍呈现0/就绪的展示风险，未作为独立确认BUG |
| UI-L06 | LogsPage / useLogs / GetLogs | GET全量日志 → 客户端级别/来源/排序/限量 → 文本过滤/分段渲染 | 匹配总数错误见 UI-04；GET未实现服务端limit |
| UI-L07 | 清空日志按钮 | 二次确认 → DELETE /logs → Logger.Clear → 失效缓存 | 已追踪；未清空用户日志 |
| UI-L08 | ThemeToggle / AppearanceSection | next-themes/theme 与自定义 ovh-theme 各自写html.dark | 两套存储/状态冲突见 UI-03 |
| UI-L09 | LanguageToggle / i18n / apiMessage | 手动语言落盘；浏览器语言回退；后端错误码插值与原文回退 | 部分导航和仪表盘硬编码中文属本地化不完整，未当业务故障 |
| UI-L10 | MonitorSubscribeDialog / AddVPSDialog | Telegram验证 → 新增按钮门禁；后端 AnyAvailable 支持任一通知渠道 | Webhook-only 被拦截见 UI-01 |
| UI-L11 | TelegramOrderPage | 原始命令或结构化字段 → /telegram/quick-order → 文本结果 → localStorage命令历史 | 自定义型号可输入；实际购买/权限判断见 order-monitor |
| UI-L12 | 云下单命令生成 | 模式/型号/机房/数量/@账户/优惠码 → 命令预览或复制；历史回填 | 历史回填未保存数量/账户等结构化字段，原始命令保留；列为待改进而不推断已错单 |
| UI-L13 | http / api facade / query keys | URL归一化 → API key/活动账户注入 → Axios → 错误码；React Query失效 | 账户隔离/认证详见其他模块；未使用的旧QueryClient默认策略不生效 |
| UI-L14 | 公共展示/错误边界 | 空态、加载、部分失败、通知、图标、响应式布局 | AST列出函数，常用页面经过真实React浏览器渲染；不是每个UI状态均已测试 |
| UI-L15 | 仪表盘同型号多配置订阅 | 卡片 → onOpenSubscribe(planCode) → Index.find → 编辑初值 | 配置身份丢失，见UI-06 |

## 已确认问题

### UI-01 · P2 · 仅配置 Webhook 时无法从界面添加独服或 VPS 订阅

- 位置：`src/components/monitor/MonitorSubscribeDialog.tsx:84-85,179,312`；`src/components/vps-monitor/AddVPSDialog.tsx:44-45,259`。
- 后端契约：`backend/internal/handlers/monitor.go:31`、`vps_monitor.go:53` 使用 `notify.AnyAvailable(state,true)`；前端已有 `src/hooks/ovh/use-notify-channels.ts:40` 的统一门禁。
- 触发：Webhook可用、Telegram未配置。界面仍只查 `/telegram/verify`，禁用确认按钮，甚至不请求 `/notify/channels`。
- 复现：浏览器mock为上述条件，两个对话框按钮均 disabled。测试中没有真实网络通知。
- 影响：后端允许的通知方式在前端无法使用；原本支持的订阅入口受阻。
- 建议：两个新增入口统一接 `useNotifyGate`，刷新/加载/失败状态明确；编辑行为继续保持既定语义。回归 Telegram-only、Webhook-only、两者无效三组。

### UI-02 · P3 · 命令面板展示的 G T / G K 快捷键没有实现

- 位置：`src/components/common/CommandPalette.tsx:33,37` 声明快捷键，但 `:85-94` 的实际映射缺少 t/k。
- 复现：在 `/logs` 按 G T / G K 均停留原页；对照 G S 正常进入 `/servers`。
- 建议：从 NAV_ENTRIES 自动导出映射，避免维护两份导航事实。

### UI-03 · P2 · 设置页选择的主题刷新后被覆盖

- 位置：`src/App.tsx:42` 的 next-themes 默认 dark（默认存储键 theme）；`src/components/settings/AppearanceSection.tsx:13` 使用自定义 hook；`src/lib/theme.ts:15,64` 写 ovh-theme。
- 复现：在设置页点击浅色，当场 html.dark=false 且 ovh-theme=light；刷新后 html.dark=true，但 ovh-theme仍为light。顶部主题选择使用另一套状态。
- 影响：明确保存的外观偏好不可靠；设置页、顶部菜单和依赖自定义hook的重装界面可能读取不同主题。
- 建议：保留单一 ThemeProvider/hook/storageKey，迁移旧值；同时测试初次访问、刷新、系统主题变化和两个入口同步。

### UI-04 · P3 · 筛选日志后总数错误，误提示存在更多匹配日志

- 位置：`src/hooks/ovh/use-logs.ts:43-82`，`src/pages/LogsPage.tsx:198-201`；后端 `backend/internal/handlers/logs.go:12-15` 无视 limit/level/source/order 返回全部快照。
- 复现：10条日志中只有1条ERROR；筛选ERROR后页面称“内存中共10条匹配日志，当前仅加载最新1条”。增加limit无法得到另外9条ERROR。
- 建议：明确API的 total/filteredTotal/returned 含义并只由一层过滤排序；短期数组兼容分支在过滤后计算total，长期后端真正实现筛选和限量。不要让后端排序后前端再次reverse。

### UI-05 · P3 · 仪表盘把本地状态当作 OVH 连通状态，且现货数引用总目录数

- 位置：`src/components/dashboard/CoreServiceCard.tsx:60-62` 的 `ok={!!stats}`；`backend/internal/handlers/stats.go:15-32` 仅读取本地状态，未探测OVH；`src/components/dashboard/OperationBanner.tsx:76,95` 的“现货”使用 `totalServers`。
- 复现：本地stats返回totalServers=10、availableServers=0，页面仍称“OVH API 连通 / 正常通信”；代码明确把总目录10显示为现货。
- 影响：运维人员可能误判外部API可达性和可购数量。顶部“后端在线”本身没有问题，错误发生在对外部OVH的推断。
- 建议：将此指示命名为“后端状态”，或使用带账户、检测时间和失败原因的真实OVH探测结果；现货统一读availableServers。

### UI-06 · P2 · 仪表盘编辑同型号第二个配置时打开第一个配置

- 位置：`src/components/dashboard/StockRadarSection.tsx:209` 的回调仅传planCode；`src/pages/Index.tsx` 的 handleOpenSubscribe、弹框initial和serverName均只按planCode查找首个订阅。
- 复现：同一audit-plan有ram-a、ram-b两个订阅，在仪表盘点击第二张卡片“设置”；弹框的配置输入实际为ram-a。浏览器测试已断言该结果。
- 影响：用户以为修改B配置，实际可修改A配置的机房、自动下单账户、数量等；监控页的配置精确识别在此入口被丢失。
- 建议：回调传完整订阅或稳定ID，至少带planCode+规范化optionsKey，Index与编辑API统一使用相同身份。测试两条同型号不同配置的查看、编辑、删除互不影响。

## 已排除与待验证边界

- `UpdateButton` 的重启状态机存在漏采样风险，但全项目只有定义没有调用；**不计为当前可达业务BUG**，详见platform。
- `TerminalConsole` 的price对象渲染可能脆弱，但实际 `/telegram/quick-order` 返回文本message并无price字段；当前API不会进入该分支，排除“实际价格查询必崩”的推断。
- `src/lib/query.ts` 创建了未被App使用的QueryClient，实际全局缓存策略来自App。属于维护债，不当成两套缓存同时写入的事故。
- API错误显示、单账户详情缓存、云下单资金/库存判定归对应模块；本报告不重复计数。
- clipboard、localStorage容量不足、浏览器私密模式、多标签快速并发未穷举；没有将假设当成复现结果。

## 复现方法

启动Vite到127.0.0.1:18080，再运行 `node docs/audit-2026-11-06/proofs/frontend-browser.cjs`。需要Playwright，可用环境变量 `PLAYWRIGHT_MODULE` 指向已有模块，`OVH_UI_BROWSER` 指向本机Chrome。脚本拦截全部API和外部访问，并拒绝任何写请求；断言的是**缺陷存在**，修复后应改为正常行为回归断言。

已运行结果：6项UI缺陷复现；另有OM-09区域错误对象场景，修复前进入ErrorBoundary，当前改为修复回归，未出现运行时异常。原多账户/自定义planCode UI回归另见测试总览。
