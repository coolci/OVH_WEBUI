# 共享领域、服务适配与工具逻辑

本分项补充主业务报告之外的独立计算逻辑。函数行号见FUNCTION-INDEX；人工文件范围见root-coverage.json。没有将未接入主流程的旧模型错误算成线上订单故障。

| 独立逻辑 | 实际职责 | 审查结论 |
|---|---|---|
| domain/catalog | 型号trim/lowercase、机房国家映射、税前税后金额 | NormalizePlanCode只有兼容引用，不能推断主下单会小写自定义型号；金额缺NaN/Inf保护，主链是否采用需逐调用判定 |
| domain/monitor | 订阅构造、间隔5–86400、全机房/指定机房、状态变化、防抖 | UpdateAvailability未见生产调用；其黑名单库存判定与主引擎白名单不同，属于旧模型语义漂移 |
| domain/order | queued/processing/success等状态机、数量1–20、毫秒间隔、重试计数 | 当前运行队列主链使用types与service/order的pending/running状态，不能混用两个模型的约束 |
| domain/system | 日志对象、指标对象、版本不等判断 | CheckUpdate仅比较字符串不等，不能判断升级/降级；真实更新入口使用handlers/version，未将此旧实现计为当前更新事故 |
| domain/telegram | 会话TTL、slash命令字符校验、消息截断 | TruncateMessage按字节截断，长UTF-8文本可能截断字符；未见生产调用，因此列未接入代码缺陷而非Bot已发生错误 |
| domain/vps | VPS实体、重建参数、密码强度、电源状态变更 | 纯内存模型；真实handler使用上游能力与请求确认，不能拿该模型替代实际OVH任务状态 |
| service/catalog | 子公司标准化→repository缓存读写清除 | 轻适配；未加入虚构的目录可售校验 |
| service/monitor | 构造领域对象进行基本校验→持久化订阅 | 验证时固定30秒且不采用归一化对象；运行态链路由实际monitor负责，需避免今后将两套模型混用 |
| service/order | 队列快照/入队/删清/重试间隔/暂停恢复/历史 | 使用QueueStore统一运行态和持久化；暂停时CancelTask；ErrPersist区分持久化失败；handler终态约束详见订单报告 |
| ValidateServiceName | 路由层服务名ASCII白名单和255字节上限 | 防止路径/查询/片段拼接污染；空service_name用于无参数路由继续执行 |
| webembed两种build tag | 无UI返回nil；ui构建嵌入web并取fs子目录 | 逻辑互斥；构建说明注释仍使用旧server/web路径，实际为backend/web |
| i18n日期和金额 | invalid日期回—、locale映射、Intl金额失败回退 | 审查了正常/非法值分支；金额已挡非有限值；语言渲染刷新由使用方负责 |
| splitList | 半全角逗号、分号、换行和tab分隔，trim空项 | 与后端兼容目标明确，非空格分隔；不应拿它处理完整带空格的自由命令 |
| clampOrderPlan | 单机房20、总扇出60、整数化 | 前端提醒，后端是权威；机房数量超过总上限时仍保留1，由后端拒绝 |
| mergeStockPlans | 目录之外的eco库存候选合并，硬件选项和prerelease标记 | 不等于售卖授权；购买链仍需校验；未知非eco型号须走自定义入口 |
| usePrewarmOrder | 配置key变化300ms防抖→预热POST→就绪状态 | 旧请求完成可能更新新配置的就绪状态，且success不检查ready；仅列展示状态风险，实际购物车key隔离应独立验证 |
| PartialList | 从header读取失败数、列表安全数组化 | 工具存在不代表所有调用方已接入；账户记录丢元数据为SS-03 |
| useActiveAccountEndpoint | 独服活动账户→默认账户→endpoint大区与能力提示 | 当前实现固定独服账户，名称注释提及VPS；复用到VPS需检查调用方是否真的使用此hook，不能仅据注释认定串账户 |
| NotFound | 未匹配路由记录控制台、返回上一页或首页 | 普通UI回退，无后端写入 |

未接入代码的差异仍建议清理或统一，但修复优先级低于已确认的真实可达资金、数据和账户状态问题。
