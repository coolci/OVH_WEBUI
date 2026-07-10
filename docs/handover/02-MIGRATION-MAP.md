# 功能迁移映射

## 源项目角色

| 源 | 角色 | 去向 |
|----|------|------|
| `ovh/server` | 完整 Go 后端 + 多账户 + 服务器/VPS 控制 | `OVH_WEBUI/backend` |
| `ovh/web` | 完整 hooks / server-control / vps-control | `src/` |
| `OVH-BUY` | 旧 Python 对照 | 仅参考契约 |
| `OVH_WEBUI` 壳 | 布局 / 移动端 | 正式前端 |

## 模块状态（当前）

| 功能 | 状态 | 备注 |
|------|------|------|
| 健康检查 / stats / metrics | ✅ | |
| 多账户 CRUD + Settings | ✅ | |
| 仪表盘 | ✅ | |
| 服务器列表 / 队列 / 历史 / 日志 | ✅ | `showApiServers=true` |
| 套餐询价 `POST /servers/:planCode/price` | ✅ | 不落单 |
| 独服 / VPS 监控 | ✅ | 含 **PUT 原地更新** |
| 服务器控制 / VPS 控制 | ✅ | |
| 账户：邮件 / **订单** / 退款 / 账单 | ✅ | orders 已补 |
| 联系人变更 | ✅ | |
| Telegram | ✅ | |
| AuthGate + ⌘K + 移动端 | ✅ | |
| **线上巡检** | ❌ **已取消** | 路由/页面/包已删 |
| **Config Sniper** | ❌ **废弃不迁** | 不加入 |

## API 路径（相对 Python 差异）

| 旧 (OVH-BUY) | 现 (Go) |
|--------------|---------|
| `/api/config` | `/api/settings` |
| `/api/servers/:pc/price` | ✅ `POST /api/servers/:planCode/price` |
| `/api/ovh/account/orders` | ✅ `GET /api/ovh/account/orders` |
| monitor 订阅更新 | ✅ `PUT /api/monitor/subscriptions/:planCode` |
| VPS 订阅更新 | ✅ `PUT /api/vps-monitor/subscriptions/:id` |
| `/api/config-sniper/*` | 无（废弃） |
| `/api/inspection/*` | 无（已取消） |
