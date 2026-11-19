/** 监控订阅的通知 / 自动下单选项 */
export type MonitorNotifyOptions = {
  notifyAvailable: boolean;
  notifyUnavailable: boolean;
  autoOrder: boolean;
  autoPay: boolean;
  quantity: number;
  autoOrderAccountIds: string[];
};

export const DEFAULT_MONITOR_OPTIONS: MonitorNotifyOptions = {
  notifyAvailable: true,
  notifyUnavailable: false,
  autoOrder: false,
  autoPay: false,
  quantity: 1,
  autoOrderAccountIds: [],
};
