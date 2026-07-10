# 架构决策记录 (ADR)

## ADR-001：后端以 Go 为准，不用 Python 重构

- **状态**：Accepted  
- **背景**：OVH-BUY 为 Flask 单文件巨石；ovh/server 已模块化且覆盖服务器/VPS 控制。  
- **决策**：`OVH_WEBUI/backend` = 从 `ovh/server` 迁入并改 module 为 `github.com/ovh-webui/server`。  
- **后果**：Config Sniper 暂缺；需按 Go 契约改前端。

## ADR-002：前端保留 OVH_WEBUI 视觉，不整站换 ovh/web

- **状态**：Accepted  
- **背景**：OVH_WEBUI 已有 dashboard 布局与终端风组件。  
- **决策**：路由/UI 以 OVH_WEBUI 为准；API/能力对齐 Go。  
- **后果**：server-control 细节组件可逐步从 ovh/web 移植。

## ADR-003：线上巡检取消

- **状态**：Accepted — **功能已移除**  
- **背景**：产品不需要独立巡检模块。  
- **决策**：删除 `/api/inspection/*`、前端页面与导航；测试与运维改用服务器控制只读接口。

## ADR-004：配置绑定狙击 (config-sniper) 完全下线

- **状态**：Accepted — **永久下线**（非二期）  
- **背景**：Go 后端已删除 config_sniper 业务表与路由；产品不再提供该能力。  
- **决策**：不迁移、不重建、前端不暴露任何入口。  
- **后果**：历史 DB 列/注释可保留兼容；业务与文档均视为废弃功能。

## ADR-005：前后端分离 + 可选 Docker 同源

- **状态**：Accepted  
- **背景**：开发效率与嵌入式 UI 二选一。  
- **决策**：默认 Vite 代理；Docker 用 nginx 反代 `/api`。  
- **后果**：无需强制 `-tags ui` 嵌入构建。

## ADR-006：统一前端 HTTP 为 axios（lib/http.ts）

- **状态**：Accepted  
- **背景**：曾并存 `api-client`（fetch）与 `http`（axios），错误处理 / `backendUrl` 不一致。  
- **决策**：
  - **唯一传输层**：`src/lib/http.ts`（axios + 鉴权 + 账户注入 + `backendUrl`）
  - **业务 facade**：`src/lib/api.ts`（语义化方法，底层 `apiRequest`）
  - **`api-client.ts`**：仅 re-export，禁止新增逻辑  
- **后果**：hooks 继续 `import { api } from "@/lib/http"`；旧页可用 `import { api } from "@/lib/api"`。
