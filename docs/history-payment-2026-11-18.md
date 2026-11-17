# 购买历史页手动支付

## 本次修复

此前 `OrderPaymentButton` 只在账户管理的订单列表使用，抢购历史只有订单链接、进度和取消按钮。本次将同一套手动支付窗口接入购买历史的桌面表格和手机卡片。自动付款保持停用。

- 下单生成订单号后即可点击“发起支付”，不必等待后台先查到 `notPaid`；窗口仍会实时检查 OVH 状态，只有当前确实未付款且存在有效绑定方式时才允许扣款。
- 付款前确认绑定下单账户、订单号、实时含税金额、币种和支付方式。任何一项变化都需重新确认；读取失败后的旧缓存不能用于授权付款。
- 已付款、放弃、过期及有在途撤单的历史不展示支付入口。正在打开的支付窗口不会因历史状态更新而被卸载，仍能查看结果。
- 打开窗口不发起扣款。只有勾选确认并点击“确认支付”才发送付款请求；不自动重试，继续复用现有持久化防重复付款保护。
- 提交付款后，窗口对仍为 `notPaid / unknown` 的在途订单每 5 秒只读查询一次，窗口关闭或隐藏时停止轮询。OVH 返回确认状态后，分别显示核验中、交付中、已交付等结果。
- 实时读取的订单状态按下单账户与订单号同步至购买历史。发送请求成功本身不会被写成“已付款”。
- 保留原订单链接，并在窗口提供“前往 OVH 网页支付 / 银行验证”。后端还会返回对应大区的 OVH 管理页链接作为兜底；无法使用绑定支付方式时，可继续在 OVH 完成支付。

## 接口核对

2026-11-18 读取 OVH 官方公开 schema，确认现有三区付款参数形状匹配接口：

- [EU /me schema](https://eu.api.ovh.com/1.0/me.json)、[CA /me schema](https://ca.api.ovh.com/1.0/me.json)：查询 `/me/order/{orderId}/paymentMethods`，按支付方式 ID 发送 `{"paymentMethod":{"id":...}}`。
- [US /me schema](https://api.us.ovhcloud.com/1.0/me.json)：查询 `/me/order/{orderId}/availablePaymentMethod`，发送 `{"paymentMethod":{"id":...,"type":...}}`。
- [OVH 订单说明](https://docs.ovhcloud.com/en/guides/account-and-service-management/managing-billing-payments-and-services/ovh-orders)：实际允许的支付方式取决于账户及地区；支付是否成功、银行验证和风控结果以 OVH 为准。

没有发起真实付款。不能用离线测试保证银行实际授权成功。

## 验证

- 前端回归新增 4 项：新建订单入口和账户绑定、不可付款订单、只读预检及在途轮询、确认金额变化及错误缓存拦截。
- 后端回归新增：只有对应账户历史更新；提交成功不冒充付款确认；OVH 确认后写回历史；只返回 registered ID 时仍能读取可用支付方式；没有绑定方式时不发起扣款。
- 复用原 EU / CA / US 支付请求格式、并发防重复、未决结果不重试、明确拒绝后重新确认等测试。
- `npm run build`、修改文件 ESLint 检查通过；`npm run test:unit:frontend` 共 95 项通过；`cd backend; go test ./internal/handlers ./internal/router -count=1` 通过。
- 重新生成接口核对快照：后端与 OpenAPI 均为 266 个操作，双方缺失项均为 0。

部署需使用新的前端构建，并重新构建、重启后端，使支付状态同步及 `managerUrl` 返回生效。
