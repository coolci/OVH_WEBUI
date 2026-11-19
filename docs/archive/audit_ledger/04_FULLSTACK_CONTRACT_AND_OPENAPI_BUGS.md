# 分册四：全栈 API 与 OpenAPI 三向契约已验证 BUG 台账（`BUG-CONTRACT-01` ~ `BUG-CONTRACT-05`）

- **所属总账**：`00_MASTER_HANDOFF_LEDGER.md`
- **收录范围**：`api/openapi.yaml`（266 个操作） ↔ `backend/internal/router/*` & `handlers/*`（264 条实装路由） ↔ `src/lib/api.ts` & `src/hooks/**`（316 处前端请求调用）三向自动化穿透对账
- **核验结果**：共 **5 项** 100% 实证契约缺陷（`Critical: 1` / `High: 3` / `Medium: 1` / `Low: 0`）
- **配套动态验证套件**：`audit_ledger/pocs/security_and_frontend/check_contracts.mjs`、全量机器对账输出 `contract_diff.json` 及 `fe_verify.test.mjs`

---

## BUG-CONTRACT-01：`api/openapi.yaml` 中 140 个独服与 VPS 控制面操作路径前缀错写为 `/servers/*` 与 `/vps/*`，与后端真实路由 `/api/server-control/*`、`/api/vps-control/*` 失配率达 52.63%

- **严重度**：`Critical` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `api/openapi.yaml:1401-3100, 4050-5050`（100 个 `/servers/...` 操作）
  - `api/openapi.yaml:3102-4045`（40 个 `/vps/...` 操作）
  - 同一文件内部自相矛盾：`api/openapi.yaml:6236-6435`（末尾新增的 9 个操作却使用 `/server-control/...` 与 `/vps-control/...`）
  - 后端真实路由：`backend/internal/router/router.go:66-72`、`backend/internal/router/routes_server_control.go`、`backend/internal/router/routes_vps_control.go`、`backend/internal/router/routes_api.go:84`
- **完整触发链路**：
  1. `api/openapi.yaml` 顶层声明 `servers: - url: /api`，故 `/servers/{service_name}/reboot` 解析为 `/api/servers/{service_name}/reboot`。
  2. 然而后端 `/api/servers` 仅用于抢购目录查询（`routes_api.go:84`），所有实例运维控制面均挂载在 `/api/server-control/*` 与 `/api/vps-control/*`。
  3. 任何依据 `api/openapi.yaml` 生成的客户端 SDK、MCP 工具或第三方自动化脚本调用这 140 个接口均 **100% 命中 HTTP 404**。
- **动态验证铁证**：
  - 通过 `cd backend && go run ./cmd/routes` 导出 264 条实装路由并与 `api/openapi.yaml` 做 AST 交叉对账（见 `contract_diff.json`）：266 个 OpenAPI 操作中精确 **140 个**因前缀错写为 `/servers/*`（100 个）和 `/vps/*`（40 个）无法命中后端（占比 **52.63%**）。
- **下一代契约规范修复方案**：
  - 统一将 `api/openapi.yaml` 所有实例运维路径修正为 `/server-control/...` 与 `/vps-control/...`，并在 CI 中将 `go run ./cmd/routes` 输出与 OpenAPI 路径做 100% 双向集合相等门禁测试。

---

## BUG-CONTRACT-02：`src/lib/api.ts` 导出的客户端方法与后端真实契约严重错位：`api.moveIp` 源/目标服务器颠倒致 Failover IP 迁错实例，`api.toggleBurst` 与 `api.getIpmiAccess` 必返 HTTP 400

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - `src/lib/api.ts:420-432, 472-476`
  - `backend/internal/handlers/server_control_misc.go:471-499`（`MoveIP`）
  - `backend/internal/handlers/server_control_features.go:157-173`（`UpdateBurst`）
  - `backend/internal/handlers/server_control_network.go:403-408, 507-515`（`GetIPMIConsole`）
- **完整触发链路**：
  1. **`api.moveIp(serviceName, ip, targetService)` 语义颠倒**：`src/lib/api.ts:472-476` 将源服务器 `serviceName` 拼入 `/api/server-control/${serviceName}/ip/move`，把目标服务器放入请求体 `{ ip, to: targetService }`；而后端 `MoveIP`（`server_control_misc.go:478-493`）仅绑定 `{ IP string }`，完全忽略 `body.to`，直接把 URL 中的 `:service_name` 当作迁移**目的地**调用 OVH `POST /ip/{ip}/move` `{ to: svc }`——想把 IP 从 A 迁往 B 时实际向 A 发起迁移！
  2. **`api.toggleBurst` 字段错名**：发送 `{ enabled: enable }`，后端 `UpdateBurst` 要求 `{ status: "active" | "inactive" | "inactiveLocked" }`，必返 `400 E8C3A474D ("缺少status参数")`。
  3. **`api.getIpmiAccess` 枚举错名**：限定 `"html5" | "jnlp" | "sol"`，后端 `GetIPMIConsole` 仅接受 `kvmipHtml5URL` / `kvmipJnlp` / `serialOverLanURL` / `serialOverLanSshKey`，必返 `400`。
- **动态验证铁证**：`fe_verify.test.mjs` 断言核验通过。
- **下一代契约规范修复方案**：
  - 废除手工双轨维护的 `src/lib/api.ts` 冗余封装，基于单一权威 OpenAPI 规范自动生成强类型 TypeScript SDK；后端 `MoveIP` 显式校验或绑定目标实例参数防呆。

---

## BUG-CONTRACT-03：`api/openapi.yaml` 含 3 条后端从未实现的幽灵路由、2 条固定返回 `410 Gone` 的已下线路由仍标 `200 OK`，且漏登多条实装路由

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  - 幽灵路由：`api/openapi.yaml:4981-5033`（`POST .../virtual-network-interface/{uuid}/disable|enable`）、`api/openapi.yaml:5173-5199`（`POST /accounts/proxy/test`，实装路径为 `routes_api.go:79` 的 `POST /api/accounts/:id/proxy-test`）
  - 固定 `410` 仍标 `200`：`api/openapi.yaml:3604-3651` vs `backend/internal/handlers/vps_control_power.go:121-127`（`VpsSetPassword` 固定返 `410 E2D28CDC9`）、`backend/internal/handlers/vps_control_misc.go:289-295`（`DeleteVpsOption` 固定返 `410 ED28E63E8`）
  - 实装漏登记：`backend/internal/router/routes_api.go:26, 79`（`GET /api/health`、`POST /api/accounts/:id/proxy-test`）及 `routes_server_control.go` 中 `DELETE .../backup-ftp/access` 等
- **动态验证铁证**：`check_contracts.mjs` 全量路由比对 0 匹配确认（见 `contract_diff.json`）。
- **下一代契约规范修复方案**：
  - 移除幽灵路由，修正 `/accounts/{id}/proxy-test`，将已退役接口标注 `deprecated: true` + `'410'`，补齐全部实装端点。

---

## BUG-CONTRACT-04：`api/openapi.yaml` 至少 9 处核心写操作 `requestBody` 字段名与后端 `ShouldBindJSON` 结构体冲突，按文档传参 100% 返 HTTP 400

- **严重度**：`High` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  1. VPS 续费：`api/openapi.yaml:3812-3842`（`{automatic, deleteAtExpiration, forced, period}`）vs `backend/internal/handlers/vps_control_basic.go:316-365`（要求 `{mode, period}`，返 `400 EA464F1AF`）
  2. 独服启动模式：`api/openapi.yaml:1818-1842`（`{bootMode: string}`）vs `backend/internal/handlers/server_control_boot.go:323-330`（要求 `{bootId: int64}`，返 `400 E81B66FDC`）
  3. 维护任务排期：`api/openapi.yaml:1698-1728`（`{timeslot}`）vs `backend/internal/handlers/server_control_basic.go:1473-1501`（要求 `{startDate}`，返 `400 E95D31CE8`）
  4. 硬件更换工单：`api/openapi.yaml:1984-2011`（`{component}`）vs `backend/internal/handlers/server_control_hardware.go:870-890`（要求 `{componentType, disks}`，返 `400 E9871712B`）
  5. 新增 SPLA：`api/openapi.yaml:4869-4893`（`{type, serial}`）vs `backend/internal/handlers/server_control_misc.go:704-723`（要求 `{type, serialNumber}`）
  6. 队列重试间隔：`api/openapi.yaml:353-381`（`{interval}`）vs `backend/internal/handlers/queue.go:203-212`（要求 `{retryInterval}`）
  7. 独服重装：`api/openapi.yaml:1576-1606`（平面 `{hostname, sshKeyName}`）vs `backend/internal/handlers/server_control_basic.go:754-795`（嵌套 `details.customHostname`、`details.sshKeyName`、`storageConfig`）
  8. 承诺期销毁闸门：`api/openapi.yaml:3323-3347, 4246-4267`（漏 `confirm`）vs `backend/internal/handlers/vps_control_engagement.go:197-200`（`CANCEL_SERVICE` 缺 `confirm: true` 返 `400 EAFF9E887`）
  9. VPS 快照回滚闸门：`api/openapi.yaml:3950-3965`（无 `requestBody`）vs `backend/internal/handlers/vps_control_snapshot.go:147-157`（缺 `{confirm: true}` 返 `400 EFD5D37BB`）
- **下一代契约规范修复方案**：
  - 将后端匿名绑定结构体提炼为具名 DTO（`internal/api/dto`），以 Go DTO 单源自动生成 `api/openapi.yaml`，消除手工维护漂移。

---

## BUG-CONTRACT-05：`api/openapi.yaml` 多处 `200` 响应 Schema 将后端包装对象错声明为顶层 `array`，并将 `/verify-auth` 的 `{valid}` 错写为 `{authenticated}`

- **严重度**：`Medium` | **状态**：`OPEN_VERIFIED`
- **精确文件与行号**：
  1. `GET /accounts`：`api/openapi.yaml:32-48`（`array`）vs `backend/internal/handlers/accounts.go:167`（`{"accounts": out, "total": len(out)}`）
  2. `GET /notify/channels`：`api/openapi.yaml:5679-5694`（`array`）vs `backend/internal/handlers/monitor.go:473`（`{"channels": ..., "anyAvailable": ..., "verified": ...}`）
  3. `GET /servers/list` & `GET /vps/list`：`api/openapi.yaml:1401-1423, 3102-3118`（`array`）vs `server_control_basic.go:78` & `vps_control_basic.go:128`（`{"success": true, "servers"|"vpsList": ...}`）
  4. `GET /ovh/account/bills|email-history|orders|refunds|sub-accounts`：`api/openapi.yaml:5233-5379`（`array`）vs `backend/internal/handlers/account.go:668, 706`（`{"status": "success", "data": list, ...}`）
  5. `POST /verify-auth`：`api/openapi.yaml:5214-5232`（`{authenticated: boolean}`）vs `backend/internal/handlers/settings.go:159-174`（`{"valid": true|false}`）
- **下一代契约规范修复方案**：
  - 统一修正响应包装 Schema，并在集成测试中接入 OpenAPI Response Validator 断言 `httptest` 真实输出。
