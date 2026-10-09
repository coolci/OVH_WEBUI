/** 前后端共享业务类型（对齐 Go handlers JSON） */

export interface Stats {
  activeQueues: number;
  totalServers: number;
  availableServers: number;
  purchaseSuccess: number;
  purchaseFailed: number;
  queueProcessorRunning: boolean;
  monitorRunning: boolean;
}

export interface QueueItem {
  id: string;
  planCode: string;
  datacenter: string;
  options?: string[];
  status: string;
  createdAt?: string;
  updatedAt?: string;
  retryInterval?: number;
  retryCount?: number;
  lastCheckTime?: number;
  accountId?: string;
  autoPay?: boolean;
  failureCount?: number;
}

export interface LogEntry {
  id: string;
  timestamp: string;
  level: string;
  message: string;
  source: string;
}

export interface HistoryPhaseTiming {
  name: string;
  ms: number;
}

export interface HistoryEntry {
  id: string;
  taskId?: string;
  accountId?: string;
  planCode: string;
  datacenter: string;
  options?: string[];
  status: string;
  orderId?: string | null;
  orderUrl?: string | null;
  orderStatus?: string | null;
  orderStatusAt?: string | null;
  expirationTime?: string | null;
  retractionTime?: string | null;
  errorMessage?: string | null;
  purchaseTime: string;
  attemptCount?: number;
  timing?: HistoryPhaseTiming[];
  totalMs?: number;
  price?: {
    withTax?: number;
    withoutTax?: number;
    tax?: number;
    currency?: string;
    currencyCode?: string;
  };
}

/** 监控状态：后端 snake_case 为准，camelCase 仅兼容旧类型引用 */
export interface MonitorStatus {
  running: boolean;
  subscriptions_count?: number;
  check_interval?: number;
  known_servers_count?: number;
  region_issues?: string[];
  /** @deprecated 后端字段为 subscriptions_count */
  subscriptionCount?: number;
  /** @deprecated 后端字段为 check_interval */
  checkInterval?: number;
}

export interface Subscription {
  planCode: string;
  datacenters?: string[];
  options?: string[];
  notifyAvailable?: boolean;
  notifyUnavailable?: boolean;
  serverName?: string;
  lastStatus?: Record<string, string>;
  history?: unknown[];
  autoOrder?: boolean;
  quantity?: number;
  autoOrderAccountId?: string;
  /** @deprecated 兼容旧字段，推荐使用 autoOrderAccountId */
  accountId?: string;
  autoPay?: boolean;
  createdAt?: string;
}

export interface VpsSubscription {
  id: string;
  planCode: string;
  displayName?: string;
  ovhSubsidiary?: string;
  datacenters?: string[];
  monitorLinux?: boolean;
  monitorWindows?: boolean;
  notifyAvailable?: boolean;
  notifyUnavailable?: boolean;
  lastStatus?: Record<string, unknown>;
  autoOrder?: boolean;
  quantity?: number;
  autoOrderAccountId?: string;
  /** @deprecated 兼容旧字段 */
  accountId?: string;
  autoPay?: boolean;
  os?: string;
  retired?: boolean;
  createdAt?: string;
  history?: unknown[];
}

export interface ServerPlan {
  planCode: string;
  name?: string;
  cpu?: string;
  memory?: string;
  ram?: string;
  storage?: string;
  bandwidth?: string;
  price?: number;
  currency?: string;
  datacenters?: Array<{ datacenter: string; availability: string }>;
  availableOptions?: Array<{ label: string; value: string }>;
  [key: string]: unknown;
}

export interface ManagedServer {
  serviceName: string;
  name: string;
  commercialRange?: string;
  datacenter: string;
  state: string;
  monitoring?: boolean;
  reverse?: string;
  ip: string;
  os?: string;
  status?: string;
}

export interface OVHAccount {
  id: string;
  name: string;
  endpoint: string;
  zone: string;
  appKey: string;
  appSecret: string;
  consumerKey: string;
  iam: string;
  isDefault: boolean;
  createdAt: string;
}

export interface ContactChangeRequest {
  id: number;
  serviceDomain?: string;
  askingAccount?: string;
  contactType?: string;
  fromAccount?: string;
  toAccount?: string;
  state?: string;
  dateDone?: string | null;
  dateRequest?: string;
  [key: string]: unknown;
}

export interface AccountInfo {
  nichandle?: string;
  email?: string;
  firstname?: string;
  firstName?: string;
  name?: string;
  country?: string;
  city?: string;
  zip?: string;
  address?: string;
  organisation?: string;
  currency?: string | { code?: string; symbol?: string };
  ovhSubsidiary?: string;
  state?: string;
  [key: string]: unknown;
}

