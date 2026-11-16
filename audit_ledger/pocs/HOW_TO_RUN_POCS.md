# 动态验证套件复现与下一代回归门禁执行指南（How to Run PoCs）

为确保主工作区源码 **0 改动、0 编译/Lint 污染**，所有 Go 测试文件均以 `.go.poc` 后缀归档在 `audit_ledger/pocs/` 下；Node.js 套件（`.mjs`）可直接在项目根目录零侵入运行。

---

## 一、零侵入直接运行：前端核心、控制面与安全契约验证套件

在仓库根目录 `c:\Users\video\Desktop\OVH_WEBUI` 打开终端直接执行：

### 1. 运行前端与配置安全复现单测（`fe_verify.test.mjs`）
```powershell
node --test audit_ledger/pocs/security_and_frontend/fe_verify.test.mjs
```
- **覆盖验证项**：`BUG-CONTRACT-01`、`BUG-CONTRACT-02`、`BUG-FE-CORE-01`、`BUG-FE-CORE-02`、`BUG-FE-CORE-03`、`BUG-FE-CORE-04`、`BUG-FE-CORE-05`、`BUG-FE-CORE-07`（精确检出 40 个缺失英文错误码）、`BUG-FE-CORE-10`、`BUG-FE-CTRL-01`、`BUG-FE-CTRL-02`、`BUG-FE-CTRL-04`、`BUG-SEC-12`。
- **历史实测快照**：见 [`security_and_frontend/fe_verify_output.txt`](./security_and_frontend/fe_verify_output.txt)（`8/8 PASS`）。

### 2. 运行全栈三向路由与契约穿透对账器（`check_contracts.mjs`）
```powershell
node audit_ledger/pocs/security_and_frontend/check_contracts.mjs
```
- **原理**：实时调用 `cd backend && go run ./cmd/routes` 导出 264 条实装路由，与 `api/openapi.yaml`（266 个操作）及 `src/` AST 提取的 316 处前端调用做三向归一化比对，刷新生成 [`security_and_frontend/contract_diff.json`](./security_and_frontend/contract_diff.json)。

---

## 二、隔离 Worktree 零污染运行：后端与安全 14 套 `go test` 动态验证套件

由于 Go 同目录下不同包名会触发编译报错，所有 `.go.poc` 文件均按各自所属内部包归档。推荐使用临时 `git worktree` 一键挂载并执行全量复现，主工作区全程零触碰：

### 1. 各 `.go.poc` 文件与后端目录精确映射表

| 归档 PoC 文件 | 目标挂载路径（相对于 Worktree 根） | 覆盖台账编号 |
| :--- | :--- | :--- |
| `security_and_frontend/sec_verify_test.go.poc` | `backend/internal/secverify/sec_verify_test.go` | `BUG-SEC-01` ~ `BUG-SEC-07`, `BUG-SEC-09`（含 `REJECTED-02`/`04` 证伪断言） |
| `backend/purchase_bugverify_ord_test.go.poc` | `backend/internal/purchase/bugverify_ord_test.go` | `BUG-OVH-01`, `BUG-OVH-02` |
| `backend/catalog_bugverify_ord_test.go.poc` | `backend/internal/catalog/bugverify_ord_test.go` | `BUG-OVH-05`, `BUG-OVH-06`, `BUG-OVH-14` |
| `backend/price_bugverify_ord_test.go.poc` | `backend/internal/price/bugverify_ord_test.go` | `BUG-OVH-04`, `BUG-OVH-10` |
| `backend/ovh_bugverify_ord_test.go.poc` | `backend/internal/ovh/bugverify_ord_test.go` | `BUG-OVH-03`, `BUG-OVH-11` |
| `backend/vps_bugverify_ord_test.go.poc` | `backend/internal/vps/bugverify_ord_test.go` | `BUG-OVH-07`, `BUG-OVH-08` |
| `backend/monitor_bugverify_ord_test.go.poc` | `backend/internal/monitor/bugverify_ord_test.go` | `BUG-OVH-09`, `BUG-OVH-15` |
| `backend/catalogwatch_bugverify_ord_test.go.poc` | `backend/internal/catalogwatch/bugverify_ord_test.go` | `BUG-OVH-13` |
| `backend/numconv_bugverify_ord_test.go.poc` | `backend/internal/numconv/bugverify_ord_test.go` | `BUG-OVH-15` |
| `backend/schemacheck_bugverify_ord_test.go.poc` | `backend/internal/schemacheck/bugverify_ord_test.go` | `BUG-OVH-17` |
| `backend/app_bugverify_ctrl_test.go.poc` | `backend/internal/app/bugverify_ctrl_test.go` | `BUG-BE-01`, `BUG-BE-04` |
| `backend/db_bugverify_ctrl_test.go.poc` | `backend/internal/db/bugverify_ctrl_test.go` | `BUG-BE-02`, `BUG-BE-07`, `BUG-BE-08`, `BUG-SEC-06` |
| `backend/telegram_bugverify_ctrl_test.go.poc` | `backend/internal/telegram/bugverify_ctrl_test.go` | `BUG-BE-11` |
| `backend/handlers_bugverify_all_test.go.poc` | `backend/internal/handlers/bugverify_all_test.go` | `BUG-BE-03`, `BUG-BE-05`, `BUG-BE-10`, `BUG-BE-13`, `BUG-OVH-16`, `BUG-SEC-04` |

### 2. PowerShell 一键隔离复现与自动清理脚本

```powershell
$repo = "c:\Users\video\Desktop\OVH_WEBUI"
$wt   = Join-Path $env:TEMP "ovh-webui-poc-verify"

# 1. 创建独立临时 git worktree（不影响主仓库）
git -C $repo worktree add --detach $wt HEAD

try {
    # 2. 挂载安全 PoC
    New-Item -ItemType Directory -Force -Path "$wt\backend\internal\secverify" | Out-Null
    Copy-Item "$repo\audit_ledger\pocs\security_and_frontend\sec_verify_test.go.poc" `
              "$wt\backend\internal\secverify\sec_verify_test.go"

    # 3. 挂载全部 13 个后端包级 PoC
    Get-ChildItem "$repo\audit_ledger\pocs\backend\*.go.poc" | ForEach-Object {
        if ($_.Name -match '^([a-z0-9]+)_(bugverify_.+\.go)\.poc$') {
            $pkg  = $Matches[1]
            $dest = Join-Path "$wt\backend\internal\$pkg" $Matches[2]
            Copy-Item $_.FullName $dest
        }
    }

    # 4. 执行全套动态验证测试
    Push-Location "$wt\backend"
    go test -v -count=1 `
        ./internal/secverify `
        ./internal/purchase `
        ./internal/catalog `
        ./internal/price `
        ./internal/ovh `
        ./internal/vps `
        ./internal/monitor `
        ./internal/catalogwatch `
        ./internal/numconv `
        ./internal/schemacheck `
        ./internal/app `
        ./internal/db `
        ./internal/telegram `
        ./internal/handlers `
        -run "TestSEC|TestBugVerify"
    Pop-Location
} finally {
    # 5. 销毁临时 worktree，保持环境绝对干净
    git -C $repo worktree remove --force $wt
}
```

---

## 三、Phase-2 改造阶段如何将 PoC 转化为永久 CI 回归测试

1. 当前归档的所有 PoC 测试断言的是**“缺陷在未修复源码上 100% 稳定触发（`PASS` 代表漏洞被成功复现）”**。
2. 在 Phase-2 每完成一个波次（Wave）修复后：
   - 将对应包的 `.go.poc` 重命名为正式 `_regression_test.go` 迁入 `backend/internal/<pkg>/`；
   - 将断言方向从“断言缺陷触发”反转为“断言修复后的安全/正确不变量成立”；
   - 更新 [`../verified_bugs_manifest.json`](../verified_bugs_manifest.json) 中对应条目的 `status` 为 `"FIXED_VERIFIED"`。
