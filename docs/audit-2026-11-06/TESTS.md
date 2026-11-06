# 修复后测试与证据

2026-11-06。本页记录当前正确行为回归；发现时的错误证明日志保留为历史，不能当作当前结果。

| 检查 | 结果与范围 |
|---|---|
| npm run verify | 通过：TypeScript、Vite构建、24项前端单测、全部backend Go包、tools/status单测、Python隔离工具测试、两份ledger |
| go vet ./backend/... ./tools/status/... | 通过 |
| 原订阅浏览器测试 | 多账户默认全选/子集/旧数据、完整自定义planCode/选配、服务器卡片、键盘、区域错误与精确暂停通过 |
| frontend-browser.cjs | TG-only/Webhook-only/无有效渠道；快捷键、主题刷新及系统变化、日志计数、后端状态、第二配置身份通过 |
| platform-browser.cjs | 已保存但验证失败422不会再次创建账户，通过 |
| test-account-records-ui.cjs | 首次错误、部分数据、保留缓存的刷新错误、真正空列表通过 |
| 平台故障注入 | 配置与VPS拒写不发布运行态、20路VPS并发新增、删除后启动、首次迁移失败保护、配对SQL错误、账户校验通过 |
| 生命周期 | localhost在途HTTP阻塞期间Run不提前返回，通过 |
| 服务器维护 | 快照确认载荷与handler；异常状态保留；详情失败；重装保留冲突拒绝/合法最终载荷；第二IP提交通过 |
| Telegram | mock401/409/429/503更新健康状态、退避且及时退出，通过 |
| 部署/SSH/重建 | 仅AST、mock SSH、真实shell隔离片段及临时PowerShell harness；没有连接/部署/编译或停止真实进程 |
| ledger | 254路由全登记，0遗漏/陈旧；假方法、错契约和无关测试不能冒充done；逐接口证据不足仍wip |
| race detector | 未成功运行：本环境CGO/C编译器条件不满足 |

最新完整输出：[fix-verification.log](evidence/fix-verification.log)。[current-verification.json](evidence/current-verification.json)为观察到的退出结果摘要。[worktree-snapshot.json](evidence/worktree-snapshot.json)仅记录源文件hash，不包含凭据内容。

## 复跑

在仓库根目录运行：

- npm run verify
- go vet ./backend/... ./tools/status/...
- powershell -ExecutionPolicy Bypass -File docs/audit-2026-11-06/proofs/run-platform.ps1
- powershell -ExecutionPolicy Bypass -File docs/audit-2026-11-06/proofs/run-server-support.ps1
- powershell -ExecutionPolicy Bypass -File docs/audit-2026-11-06/proofs/run-order-monitor.ps1
- node --test docs/audit-2026-11-06/server-support-proof.mjs

浏览器测试需要localhost Vite（默认127.0.0.1:18080）、Playwright和Chrome；PLAYWRIGHT_MODULE、OVH_UI_BROWSER、OVH_UI_BASE可指定路径。四个脚本：scripts/test-subscription-ui.cjs、scripts/test-account-records-ui.cjs、proofs/frontend-browser.cjs、proofs/platform-browser.cjs（后两者位于本目录）。所有API和外部访问均拦截。

## 历史与限制

verify.log、platform-current.log、server-current.log、tooling-results.log等旧审计日志仍保存发现时结果，名称中的current仅相对于当时审计。旧platform_test.go、server_support_test.go、telegram_poller_audit_test.go.txt仅供重建基线，不应再次整体overlay到修复代码；当前run-*脚本已改为正式回归。

没有真实下单、扣费、磁盘重装、工单发送、SSH或生产部署。不要用smoke_test.py、full_functional_test.py或部署脚本代替本次隔离回归：它们可能创建购物车、连接真实主机或发送写请求。历史凭据轮换未执行。

ESLint历史基线225 errors/23 warnings未作为业务BUG数，也未全面清理。构建的大chunk及浏览器数据库过旧提示仍在。离线测试不保证上游库存、鉴权和实际交付永不失败。
