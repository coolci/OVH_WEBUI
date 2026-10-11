# OVH_WEBUI 工程规范验证 — 2026-10-12

结论：**部分工程基础已落地，但当前工作区未通过工程质量验收，不能沿用“全量解耦、契约 100% 覆盖、质量门禁 100% PASS”的完成声明。**

本次验证以本地 `main`、HEAD `93f1e90` 及检查时的工作区为对象。核对依据包括 `docs/handover/`、两份进度台账、`api/openapi.yaml`、构建与检查配置。PRD v3 中 Go/Templ、双进程等迁移目标单独看待，没有把尚未迁移的技术选型直接判作当前运行故障。

审计未修改业务源码、既有未提交修改或运行数据，未执行真实 OVH 写操作、部署、服务重启及实机烟测。后端编译失败涉及检查开始前已有的未提交修改，不能据此判断 HEAD 提交本身也有相同问题。检查期间工作区还出现了新的未跟踪前端文件，以下结果是各检查执行时的快照。

## 实测结果

| 检查 | 结果 | 说明 |
|---|---|---|
| 后端 `go test ./...` | 失败 | `internal/handlers/telegram.go:177:14: undefined: db`；主包、handlers、router、worker 无法编译 |
| 后端 `go vet ./...` | 失败 | 同一编译错误阻断 |
| 后端 `go test -count=1 ./internal/domain/... ./internal/service/... ./internal/repository ./internal/platform/...` | 通过 | 局部包通过不等于完整后端通过 |
| 前端 `npm run lint` | 失败 | 211 errors、21 warnings |
| `tsc --noEmit --project tsconfig.app.json --incremental false` | 失败 | 5 个类型错误，见下文 |
| `tsc --noEmit --project tsconfig.node.json --incremental false` | 通过 | Node/Vite 配置类型检查 |
| `npm run build -- --outDir <临时目录>` | 通过，有警告 | 9.65 秒、3664 modules；主 JS 约 1,861.47 kB，gzip 563.36 kB，触发大 chunk 警告；浏览器兼容数据库过期警告 |
| `python scripts/check-encoding.py` | 通过 | 使用 Codex 捆绑 Python 执行，632 个项目文本文件未发现非法 UTF-8 或已知乱码 |
| 后端 `gofmt -l .` | 非空 | 91 个文件需要格式化；未执行格式化写入 |
| `go run ./tools/status/main.go --ledger docs/prd-v2/progress/ledger.csv --verify` | 表面通过 | 输出 done 228、mismatches 0，但检查实现不足以证明 DoD |
| `go run ./tools/status/main.go --ledger progress/ledger.csv --verify` | 有 222 项不一致 | **仍返回退出码 0**，无法作为可靠门禁 |
| 文档原命令 `go run ./tools/status --ledger progress/ledger.csv --verify` | 失败 | 根目录没有 go.mod/go.work，提示 cannot find main module |
| OpenAPI 基础解析 | 通过 | YAML 可解析、191 paths、228 operations，无重复 operationId、无未解析内部引用；台账 target 的方法与路径均存在 |
| OpenAPI 与路由注册静态对照 | 不一致 | 静态枚举 246 个 HTTP 路由；方法与规范化路径直接相符 85 项，161 个实际路由未直接对应，143 项契约未直接注册 |

OpenAPI 统计按实际 `servers` 前缀展开，参数名称统一为占位符后比较；没有擅自将 `/servers` 映射为 `/server-control`。这是源码静态对照，不是启动服务后的端到端测试，也不是完整 OpenAPI 3.1 语义校验。

## 主要问题及证据

### 1. P1：当前工作区后端不能构建

`backend/internal/handlers/telegram.go:177` 仍使用 `db.ParseTelegramButtonOptions`，但该文件既有未提交 diff 删除了 db 的 import。完整测试与 vet 均因此失败。应先协调并完成这处进行中的重构，再重跑全量检查；不要以领域层局部测试通过替代后端构建通过。

### 2. P1：前端生产构建没有类型门禁，构建成功掩盖类型错误

`package.json:8` 的 build 仅为 `vite build`，`Dockerfile.frontend:18` 直接调用它。实际类型错误为：

- `src/components/dashboard/CoreServiceCard.tsx:4`：导入不存在的 `StatsResponse`。
- `src/components/dashboard/CoreServiceCard.tsx:5`：导入不存在的 `UpdateCheckResult`。
- `src/components/dashboard/OperationBanner.tsx:3`：导入不存在的 `StatsResponse`。
- `src/components/server-control/ReinstallDialog.tsx:340`：`OSTemplate[]` 不满足子组件要求的字符串索引签名。
- `src/components/server-control/ReinstallDialog.tsx:341`：分组模板数组受同一类型不兼容影响。

相关实际导出名称为 `DashboardStats` 和 `UpdateCheck`，子组件类型见 `src/components/server-control/reinstall/OsTemplateSection.tsx:21`。应先修正类型边界，并让正式构建前执行类型检查。进度文档中的“打包成功、0 TS 错误”不能作为独立类型检查的证据。

### 3. P1：台账验证会产生错误的完成结论

`tools/status/main.go` 当前实现与台账 README 所要求的“从代码与契约重算 DoD”不相符：

- 第 187 行：只做字符串包含检查，没有解析方法、参数或响应契约。
- 第 231 行：`hasLayeredImplementation` 只判断领域目录存在，未验证端点执行路径或架构依赖。
- 第 240 行：`hasPassingTest` 只判断目录内存在任意 `_test.go`，不执行测试，也不建立端点与测试映射。
- 第 81 行：发现不一致只打印日志，不以非零退出码终止。
- 第 44 行：忽略 OpenAPI 读取错误。

所以即使后端无法编译，工具仍会报告 228/228 done。根台账已有 222 项不一致仍退出 0，是已实测的门禁失效。应使用可执行测试结果、真实路由及契约校验作为证据，并在不一致或输入损坏时失败；在修复前，不宜据此自动更新 done 状态。

### 4. P1：契约覆盖数量与当前 API 一致性混为一谈

`api/openapi.yaml:7` 声明当前 API 前缀 `/api`，但 `:1371` 使用 `/servers/list`、`:3072` 使用 `/vps/list`。实际分别注册为 `/api/server-control/list` 和 `/api/vps-control/list`，见 `backend/internal/router/routes_server_control.go:12`、`:17` 和 `routes_vps_control.go:12`、`:17`。设备配对、支持工单等实际路由也未进入当前契约。

响应语义也有差异：`api/openapi.yaml:47` 将 GET `/accounts` 的响应定义为数组，而 `backend/internal/handlers/accounts.go:167` 实际返回 `{accounts, total}`；POST `/accounts` 契约写 201 + Account，`:269` 实际返回 200 + 包装对象。

若这些路径是迁移后的目标，应明确标识为目标契约、保留当前契约，并在真实路由迁移前维持 wip。仅证明台账 target 都在 YAML 中，不能证明当前接口全部受契约约束。

### 5. P1：推荐备份流程遗漏数据库解密密钥

`docs/DEPLOY.md:100` 只指导备份 `ovh_webui_data` 卷。但一键部署在 `scripts/linux-oneclick-deploy.sh:133` 将 `OVH_DB_KEY` 写入宿主机根 `.env`，再由 `docker-compose.https.yml:33` 注入容器。`backend/internal/secret/secret.go:121` 发现环境变量密钥后直接使用，不向 `/data/.env` 写一份。

因此，首次采用该部署方式时，若宿主机 `.env` 丢失，仅恢复文档中的数据卷备份不能解密已保存的 OVH 凭据与 Telegram Token。`README.md:99` 对“包括密钥在内全在 /data”的描述也不适用于该流程。应把对应密钥配置纳入备份与恢复验证，不在审计材料中记录真实密钥。

### 6. P2：领域层与统一错误体系仅部分接入

真实进展是 handlers 已不直接访问 `state.DB` 或写业务 SQL，订单服务也通过 `QueueStore/HistoryStore` 操作状态。但以下证据不足以支撑“全领域 DDD 闭环”：

- `service/account/account_service.go:143`、`service/catalog/catalog_service.go:42`、`service/server/server_service.go:45` 仅以 `var _ = domain...` 占位引用领域模型。
- `domain/order`、`domain/vps`、`domain/telegram`、`domain/system` 无生产代码导入。领域订单状态及时间范围也与在用 DTO 不同，见 `domain/order/order.go:15` 与 `types/types.go:36`、`:156`。
- `repository/interfaces.go:8`、`:23` 仍依赖具体 db 包及其 DTO，因此仓储接口并未完全脱离具体持久层。

错误方面，非测试 handlers 至少 217 处仍使用旧 `success/error` 响应，例如 `backend/internal/handlers/vps_control_power.go:27`、`util.go:304`。`api/openapi.yaml:5850` 约定 `application/problem+json`，但 `backend/internal/platform/httperr/problem.go:47` 使用普通 `c.JSON`。应分域完成真实接入与响应契约测试，再宣布全量完成。

### 7. P2：已配置的静态规范未通过，且没有仓库内 CI 强制

ESLint 实测 211 个错误包含 208 个 `no-explicit-any`、2 个 `no-empty-object-type`、1 个 `no-require-imports`；21 个警告包含 Fast Refresh 和 Hooks 依赖问题。Go 格式检查另有 91 个文件输出。

仓库没有 `.github` 或其他已跟踪的常见 CI 流水线；`package.json:17` 的后端测试入口也只覆盖 catalog/numconv 两包。应建立统一 verify 入口，接入类型检查、lint、完整 Go 测试、编码与契约校验。这里只确认本地仓库没有对应流水线，没有核查远程托管设置或分支保护。

`tsconfig.app.json` 关闭 strict/noImplicitAny、ESLint 关闭 unused 检查，说明类型保障仍较宽松；在缺乏明确项目约束时，不将这些选择单独列为违规。gofmt 同样作为通用 Go 基线报告，未声称仓库已明确设为强制门禁。

### 8. P2：交付入口与文档缺乏一致、可复现的基线

- `Dockerfile.frontend:9` 在 `npm ci` 失败后回退 `npm install` 并隐藏前者日志，绕过锁文件一致性检查；应保留失败并停止构建。
- `README.md:46`、`:118`、`:123` 等仍使用不存在的 `server/`、`web/`；`:125` 的 ldflags 模块路径也仍为上游模块。当前工程是 `backend/ + src/`，模块在 `backend/go.mod:1`，Go 最低版本为该文件声明的 1.25.0。
- `docs/DEPLOY.md:89` 引用旧 ssl 叠加编排，旧 `docker-compose.ssl.yml:46` 检查 Caddy 管理端口，而 `deploy/Caddyfile:8` 已 `admin off`；静态配置显示该健康检查不会成功。推荐一键脚本实际使用另一份 `docker-compose.https.yml`。
- `docs/handover/00-INDEX.md` 声称唯一官方记忆层，`progress/README.md` 又声明根台账为事实源，而工具默认优先 `docs/prd-v2/progress/ledger.csv`。两个台账已经发生 222 项状态差异。应指定一个当前事实源并将旧版归档。

## 已验证的基础与修复次序

已具备：前后端模块目录、服务与仓储接口、订单状态存储抽象、部分真实领域规则接入、兼容 HTTP 客户端仅 re-export、编码配置与检查脚本、若干有效的单元测试、锁文件、多阶段 Docker 构建和秘密文件忽略规则。OpenAPI 本身也已具备 228 项方法/路径条目。

建议依次处理：

1. 恢复当前后端编译，修复 5 个前端类型错误，清理阻断 lint 的问题。
2. 修正 verify 的退出码和证据判断，接入统一检查入口与实际 CI。
3. 对齐真实路由、响应、测试与契约；统一台账来源，按可复现证据重新评估完成度。
4. 补齐加密密钥备份和恢复验证，统一部署文档与编排。
5. 按真实业务执行路径逐域完成领域规则与错误体系接入，并处理格式化等维护性问题。

此次结论是工程规范审计，不代表线上服务当前不可用，也没有据此修改运行中的程序。
