/**
 * Server Control Hooks Facade
 * 遵循 Google 前端工程规范将原本 74KB 的巨型 hook 按功能域解耦为高内聚子模块：
 * - types: 领域实体与交互模型
 * - use-server-core: 基础机器、服务信息、续费与到期终止
 * - use-server-hardware: 硬件、磁盘、RAID、中奖比对、BIOS、工单
 * - use-server-boot: 启动模式、任务、救援模式、重启
 * - use-server-install: 系统模板、重装与安装进度
 * - use-server-network: 弹性 IP、网络接口、DDoS Mitigation、DNS、vRack、MAC
 * - use-server-features: 监控、Burst、防火墙、备份 FTP、云备份
 * - use-server-engagement: 合同期变更、退款撤单、SPLA 授权、选项
 */

export * from "./server-control/types";
export * from "./server-control/use-server-core";
export * from "./server-control/use-server-hardware";
export * from "./server-control/use-server-boot";
export * from "./server-control/use-server-install";
export * from "./server-control/use-server-network";
export * from "./server-control/use-server-features";
export * from "./server-control/use-server-engagement";
