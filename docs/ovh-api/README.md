# OVH API 全量盘点（2026-11-08）

数据来源：OVH 官方 API 定义 `https://eu.api.ovh.com/1.0/*.json`（v1，70 个分区）与 `https://eu.api.ovh.com/v2/*.json`（v2，18 个分区），逐个下载后提取全部操作。

- 全部操作：**7290 个**（v1 + v2）
- 本项目已接入：**184 个**（以 `backend/internal/schemacheck/baseline_test.go` 的 `usedEndpoints` 为准，该清单由测试保证覆盖代码里每一个 OVH 调用）
- 完整清单：[`endpoints.csv`](endpoints.csv)（版本、分区、方法、路径、状态、是否免登录、本项目是否已用、说明）

> 本项目已用的接口里**没有**正在调用的废弃接口：OVH 2026-10 废弃的 VPS `status / templates / reinstall / setPassword / distribution` 等 9 个已登记在 `removedEndpoints`，代码已切换到新接口或给出"已下线"提示。

## 与本项目相关的分区覆盖情况

| 分区 | 接口数 | 已接入 | 废弃 | 免登录 |
|---|---:|---:|---:|---:|
| `/dedicated/server` | 131 | 77 | 8 | 2 |
| `/dedicated/installationTemplate` | 9 | 6 | 0 | 3 |
| `/vps` | 88 | 31 | 21 | 4 |
| `/order/cart` | 350 | 19 | 15 | 301 |
| `/order/catalog` | 77 | 2 | 7 | 65 |
| `/order/upgrade` | 92 | 0 | 0 | 0 |
| `/me` | 392 | 20 | 25 | 2 |
| `/services` | 49 | 9 | 0 | 0 |
| `/service` | 8 | 0 | 0 | 0 |
| `/ip` | 86 | 10 | 2 | 1 |
| `/vrack` | 77 | 0 | 6 | 0 |
| `/support` | 9 | 7 | 0 | 0 |
| `/auth` | 6 | 1 | 0 | 2 |
| `v2 /dedicated` | 9 | 2 | 0 | 0 |
| `v2 /notification` | 19 | 0 | 0 | 0 |
| `v2 /commercialCatalog` | 2 | 0 | 0 | 0 |
| `v2 /location` | 2 | 0 | 0 | 0 |
| `v2 /iam` | 31 | 0 | 0 | 0 |

其余分区（公有云、域名、邮箱、电话、托管、许可证等 70 余个）与本项目的"独服 / VPS 抢购与运维"无关，未列出，见 CSV。

## 建议接入（按价值排序）

### 第一档：直接提升抢购成功率 / 降低限流

1. **询价、查可选时长改用免登录购物车**（2026-11-08 已在 EU 正式环境实测，不带任何凭据）

   | 操作 | 免登录结果 |
   |---|---|
   | `POST /order/cart` 建车 | ✅ 200，返回 cartId，约 24 小时后过期 |
   | `GET /order/cart/{id}` 读车 | ✅ 200 |
   | `GET /order/cart/{id}/eco` 列型号与计价周期 | ✅ 200（99 个型号；周期写法为 `P1M / P1Y / P2Y`，另有 `P0D` 安装费条目） |
   | `POST /order/cart/{id}/eco` 加商品 | ✅ 200，返回 itemId 与价格 |
   | `GET /order/cart/{id}/eco/options` 列配件 | ✅ 200 |
   | `GET .../item/{itemId}/requiredConfiguration` | ✅ 200 |
   | `POST .../item/{itemId}/configuration` 写机房/系统/区域 | ✅ 200 |
   | `POST /order/cart/{id}/eco/options` 加配件 | ✅ 200 |
   | `GET /order/cart/{id}/summary` 完整报价 | ✅ 200（有货配置返回含税/不含税总价；无货配置返回 400 "is not available in xxx"，是库存错误不是登录错误） |
   | `GET /order/cart/{id}/checkout` 结账预览 | ❌ 401 You must login first |
   | `POST /order/cart/{id}/assign` 绑定账户 | ❌ 401 |
   | `POST /order/cart/{id}/checkout` 下单 | ❌ 401 |
   | `GET /order/cart` 列出购物车 | ❌ 401 |
   | `DELETE /order/cart/{id}` 删除购物车 | ❌ 401 —— **匿名车删不掉，只能等它过期** |

   现在询价、读可选购买时长（`/api/order/durations`）、监控验价都用账户签名的购物车。改成匿名车的好处是这些只读操作不再以下单账户身份请求 OVH。需要注意三点：
   - **限流是否按账户计、匿名请求是否单独计，OVH 没有公开说明，未实测**；"节省账户配额"是推断，不是已验证的结论。
   - **税额可能和账户实际下单时不同**：匿名车按子公司默认税率报价（实测 IE 子公司 23%），而账户的税率取决于账户的国家与税号（例如有欧盟税号的企业账户为 0%）。所以匿名报价适合用来判断"能不能买、有哪些周期、不含税价"，结账前的最终价格仍应以绑定账户后的报价为准。
   - 匿名车无法删除，每次询价都会在 OVH 侧留下一个约 24 小时后过期的空车（不挂在账户名下）。

2. **VPS 库存一次查全部型号**：`GET /vps/order/rule/datacenter/aggregated`（免登录，`planCodes` 可传多个）
   现在 VPS 监控每个订阅单独请求一次 `/vps/order/rule/datacenter`，订阅之间还要间隔 1 秒；改用聚合接口后同一子公司的所有订阅一次请求，检查更快、请求更少。

3. **启动时清理遗留购物车**：`GET /order/cart`（列出账户名下所有购物车，需登录）+ `DELETE /order/cart/{id}`（需登录）
   进程崩溃或强制退出时，预热车、失败车可能没删掉。OVH 侧堆积的购物车会把账户推向限流，正好在补货那一刻下不了单。

4. **自动付款前检查默认支付方式**：`GET /me/payment/method?default=true`
   开启"自动付款"但账户没有默认支付方式时，订单会建成未付款状态，促销单 15 分钟就作废。在开启自动付款时提前提示，比事后发现强。

### 第二档：下单后的体验

5. **订单交付进度**：`GET /me/order/{orderId}/followUp`（BETA）
   付款后订单在"验证 / 交付"哪一步，可以显示在购买历史里，不用去 OVH 后台看。

6. **订单 → 服务器关联**：`GET /me/order/{orderId}/associatedObject`
   交付完成后直接拿到服务器 serviceName，购买历史可以一键跳到"服务器控制"。

7. **VPS 系统可选性**：`GET /vps/order/rule/osChoices`（免登录）
   VPS 自动下单选系统时，可以先确认该机房该系统是否可用，减少结账时才失败。

8. **独服系统可选列表**：`GET /dedicated/server/osAvailabilities?hardware=`（免登录）
   重装/预选系统时按硬件过滤可用系统。

### 第三档：扩展范围（工作量较大，需要你决定）

9. **非 Eco 独服下单**：`/order/cart/{id}/baremetalServers`（BETA）与 `/order/catalog/public/baremetalServers`
   目前本项目只支持 Eco（Kimsufi / So you Start / Rise），Advance / Scale / High-Grade 会被判定为"不走 Eco"并拒绝。接入后可以抢这些系列，但下单链路、配置项和价格结构都不同，需要单独实现。

10. **按区域查独服库存**：`GET /dedicated/server/region/availabilities`（BETA，需登录）
    与现有的按机房查询互补，可用于"整个大区有货就通知"。

11. **升级类**：`/order/upgrade/vps`、`/order/upgrade/baremetalPublicBandwidth` 等（全部 BETA）
    VPS 升配、独服带宽升级的下单接口。

12. **账户安全面板**：`/me/api/credential`（列出/吊销 API 凭据）、`/me/api/logs/self`（本账户的 API 调用记录）
    可以发现凭据泄露或异常调用。

### 不建议接入

- `v2 /notification`：OVH 自己的通知路由（邮件/短信），本项目已有 Telegram / Webhook。
- `/newAccount`、`/me/accessRestriction`（两步验证管理）、`/me/payment/method` 的增删：涉及账户与支付方式变更，风险高、使用频率低。
- `v2 /iam`：细粒度权限策略，个人抢购场景用不上。
