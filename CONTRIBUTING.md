# Contributing to OVH_WEBUI

感谢参与 **OVH_WEBUI** 的开发与维护。本项目遵循 **GNU AGPL-3.0** 许可证（派生自 [`gokele/ovh`](https://github.com/gokele/ovh)），请确保所有贡献符合本规范。

## 仓库结构

```text
.
├── .github/workflows/   # CI 与发版流水线
├── server/              # Go (Gin) 后端模块（默认监听 :19998，SQLite sniper.db）
├── web/                 # React + TypeScript + Vite 前端工程（开发 :8080）
├── api/openapi.yaml     # OpenAPI 3.1 接口契约（与后端路由 100% 双向对齐）
├── deploy/              # Dockerfile、HTTPS Compose 覆盖、Nginx 与 Caddy 配置
├── docs/                # 文档、ADR、运维说明与历史归档 (docs/archive/)
├── scripts/             # 本地启动、初始化与部署脚本
└── tools/               # 状态台账校验 (tools/status)、错误码生成与辅助工具
```

## 代码规范

### Go (`server/`)
- 格式化：`gofmt` / `goimports`
- 静态检查：`go vet ./...`
- 错误包装：统一 `fmt.Errorf("...: %w", err)`，对外返回稳定错误码
- 所有 OVH API 路径拼装必须经过安全路径构造器（受 `ovh_path_safety_test.go` AST 门禁保护）
- 测试采用表驱动风格；需联网的测试遵循 `-short` 跳过约定

### TypeScript / React (`web/`)
- 路由使用 **React Router v6**（`web/src/pages/`），服务端状态统一走 **TanStack Query**
- 代码检查与类型校验：`npm --prefix web run lint`（0 warning）与 `npm --prefix web run typecheck`
- 国际化键：`web/src/i18n/locales/en.ts` 与 `zh.ts` 保持 100% 键完整对齐

### 提交规范
- 遵循 [Conventional Commits](https://www.conventionalcommits.org/)：`feat`、`fix`、`refactor`、`docs`、`chore`、`test`、`ci`

## 提交前必跑门禁

在仓库根目录执行：

```bash
npm run verify
```

分别执行各项门禁：

```bash
# 前端：ESLint + TypeScript + 构建 + 单元测试
npm run lint
npm run typecheck
npm run build
npm run test:unit:frontend

# 后端：go vet + 全量测试
cd server && go vet ./... && go test ./... -count=1

# 契约与工具链门禁
npm run test:unit:tooling
npm run verify:ledger
```
