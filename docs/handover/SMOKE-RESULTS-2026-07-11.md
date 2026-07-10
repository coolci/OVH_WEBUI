# 烟测结果 2026-07-11

## 环境

- Backend: `ovh-webui.exe` @ `:19998`
- API Key: `<api-secret-from-env>`
- Script: `scripts/smoke_test.py`（已对齐巡检下线）
- 测试机约束: **仅** `<redacted-server>` 写操作

## 结果：SMOKE PASSED（复扫后）

| 步骤 | 结果 |
|------|------|
| GET /health | PASS |
| inspection / config-sniper 404 | PASS（已下线） |
| GET /api/accounts | PASS |
| POST /api/verify-auth | PASS valid=true |
| GET /api/ovh/account/info | PASS（返回 /me） |
| GET /api/server-control/list | PASS 2 台，目标机 state=ok |
| GET /api/servers?showApiServers=true | PASS count=98 |
| GET hardware 目标机 | PASS |
| GET /api/stats | PASS |

## 全功能只读

见 [FULL-TEST-RESULTS.md](./FULL-TEST-RESULTS.md)：**54/54 PASS**。

## 结论

1. 多账户入库 + OVH 签名链路正常  
2. 巡检 / Config Sniper 正确 404  
3. 独服列表与目标机只读控制可用  
4. 未对任何机器执行 reboot/重装/terminate  
