# 测试策略（TDD + 烟测）

## 单元测试（优先纯逻辑）

```bash
cd backend
go test ./internal/catalog/ ./internal/numconv/ -count=1 -v
```

覆盖：

- catalog 标准化 / 存储解析（SAS vs SA、hybrid、0disk）
- numconv 数字转换（OVH UseNumber 坑）

```bash
# 或从 monorepo 根：
npm run test:unit:backend
```

## 烟测（需后端运行 + 真实 Key）

```powershell
$env:API_SECRET_KEY = "<backend/.env 中的密钥>"
# 可选 OVH_* 与 SMOKE_ALLOWED_SERVER，见 docs/SECURITY.md
python scripts/smoke_test.py
```

烟测步骤（当前实现）：

1. Health  
2. 断言 inspection / config-sniper **404**  
3. 列出/可选创建账户  
4. verify-auth（有账户时）  
5. account info  
6. list servers  
7. catalog  
8. 目标机 hardware（仅当设置了 SMOKE_ALLOWED_SERVER）  
9. stats / logs limit  

## 全功能只读

```powershell
$env:API_SECRET_KEY = "..."
$env:SMOKE_ALLOWED_SERVER = "..."   # 可选
python scripts/full_functional_test.py
```

- **禁止**：暂停 / 关机 / 重启 / 重装 / 删除 / 终止

## 实机范围

- 写操作目标机仅通过 `SMOKE_ALLOWED_SERVER` 约束  
- 其它机器只做列表展示，不做破坏性 API

## 前端

- 开发：`npm run dev`（Vite 代理 `/api`、`/health` → `:19998`）
- 类型：`npx tsc -b`
- 统一 HTTP：`src/lib/http.ts`（axios）
