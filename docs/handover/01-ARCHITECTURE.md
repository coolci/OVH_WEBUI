# 架构说明

## 总览

```
OVH_WEBUI/
├── backend/                 # Go 服务 (module: github.com/ovh-webui/server)
│   ├── main.go              # 路由装配、后台队列/监控
│   └── internal/
│       ├── app/             # 运行时 State（账户/队列/缓存）
│       ├── auth/            # X-API-Key 中间件
│       ├── catalog/         # 目录标准化 + 存储解析
│       ├── config/          # KV 配置
│       ├── db/              # SQLite 持久化
│       ├── handlers/        # HTTP handlers（按域拆分）
│       ├── monitor/         # 独服可用性监控
│       ├── ovh/             # OVH client 工厂（多账户）
│       ├── purchase/        # 抢购队列处理器
│       ├── telegram/        # TG 通知/下单
│       ├── types/           # 共享 DTO
│       └── vps/             # VPS 相关
├── src/                     # React 前端
│   ├── components/          # layout / dashboard / server-control / vps-control / ui
│   ├── hooks/ovh/           # React Query hooks（主路径）
│   ├── hooks/useApi.ts      # 旧 facade hooks（走 lib/api）
│   ├── lib/http.ts          # ★ 统一 axios 传输层
│   ├── lib/api.ts           # 业务 API facade（底层 → http）
│   ├── lib/api-client.ts    # 兼容 re-export → http
│   └── pages/               # 路由页面
├── scripts/                 # smoke / full_functional_test
├── docs/handover/           # 交接记忆
├── docker-compose.yml
└── nginx/                   # 前端容器反代
```

## 设计原则

| 原则 | 实践 |
|------|------|
| 模块化 | 后端按 package 分域；handlers 不直接写业务 SQL |
| 多账户 | `ovh_accounts` + `ClientFor(accountID)` |
| 安全默认 | API Key 鉴权；写操作需显式确认 |
| 可测试 | 纯逻辑包（catalog、numconv）优先 TDD |
| 前端可替换 | 所有能力经 `/api/*`；Vite 开发代理同源 |
| 单一传输层 | 仅 `lib/http.ts` 发 HTTP（axios + backendUrl） |

## 前端 HTTP 分层

| 层 | 文件 | 职责 |
|----|------|------|
| 传输 | `lib/http.ts` | axios 实例、`apiRequest`、鉴权/账户/backendUrl |
| 业务 facade | `lib/api.ts` | `api.getStats()` 等语义化方法 |
| Hooks | `hooks/ovh/*` | React Query，直接用 axios `api` |
| 兼容 | `lib/api-client.ts` | re-export http，勿新增逻辑 |

## 运行时

- 默认端口：`19998`
- 数据：`./data/sniper.db` + logs/cache
- 队列处理器：启动时 `go purchase.ProcessQueueLoop`
- 监控：有订阅时自动 Start

## 前端路由

| 路径 | 页面 |
|------|------|
| `/` | 仪表盘 |
| `/servers` | 可购服务器列表 |
| `/queue` `/history` | 抢购队列/历史 |
| `/monitor` `/vps-monitor` | 库存监控 |
| `/server-control` `/vps-control` | 已购独服 / VPS 控制 |
| `/performance` | 流量/性能 |
| `/account` `/contact-change` | 账户与联系人 |
| `/settings` `/logs` | 设置与日志 |
| `/telegram-order` | TG 下单辅助 |

> ~~`/inspection`~~ 已删除（ADR-003）。
