/**
 * OVH Server Sniper API Service
 * 与Flask后端对接的API服务层
 */

// 从localStorage读取API配置
const getApiConfig = () => {
  const backendUrl = localStorage.getItem('backendUrl') || 'http://localhost:19998';
  const apiSecretKey = localStorage.getItem('apiSecretKey') || '';
  return { backendUrl, apiSecretKey };
};

// 请求头生成器
const getHeaders = (): HeadersInit => {
  const { apiSecretKey } = getApiConfig();
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
  };
  
  if (apiSecretKey) {
    headers['X-API-Key'] = apiSecretKey;
    headers['X-Request-Time'] = Date.now().toString();
  }
  
  return headers;
};

// 通用请求方法
const apiRequest = async <T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> => {
  const { backendUrl } = getApiConfig();
  const url = `${backendUrl}${endpoint}`;
  
  const response = await fetch(url, {
    ...options,
    headers: {
      ...getHeaders(),
      ...options.headers,
    },
  });
  
  if (!response.ok) {
    const errorData = await response.json().catch(() => ({}));
    throw new Error(errorData.message || errorData.error || `HTTP ${response.status}`);
  }
  
  return response.json();
};

// ==================== 系统状态 ====================
export const api = {
  // 健康检查
  health: () => apiRequest<{ status: string }>('/health'),
  
  // 获取统计数据
  getStats: () => apiRequest<{
    activeQueues: number;
    totalServers: number;
    availableServers: number;
    purchaseSuccess: number;
    purchaseFailed: number;
    queueProcessorRunning: boolean;
    monitorRunning: boolean;
  }>('/api/stats'),
  
  // ==================== 配置管理 ====================
  getConfig: () => apiRequest<{
    appKey: string;
    appSecret: string;
    consumerKey: string;
    endpoint: string;
    tgToken: string;
    tgChatId: string;
    iam: string;
    zone: string;
  }>('/api/config'),
  
  saveConfig: (config: {
    appKey: string;
    appSecret: string;
    consumerKey: string;
    endpoint: string;
    tgToken: string;
    tgChatId: string;
    iam: string;
    zone: string;
  }) => apiRequest<{ status: string }>('/api/config', {
    method: 'POST',
    body: JSON.stringify(config),
  }),
  
  // ==================== 服务器列表 ====================
  getServers: () => apiRequest<Array<{
    planCode: string;
    name: string;
    cpu: string;
    ram: string;
    storage: string;
    bandwidth: string;
    price: number;
    currency: string;
    datacenters: Array<{
      datacenter: string;
      availability: string;
    }>;
    availableOptions: Array<{
      label: string;
      value: string;
    }>;
  }>>('/api/servers'),
  
  refreshServers: (force?: boolean) => apiRequest<{
    status: string;
    count: number;
    servers: Array<any>;
  }>(`/api/servers/refresh${force ? '?force=true' : ''}`, {
    method: 'POST',
  }),
  
  // ==================== 队列管理 ====================
  getQueue: () => apiRequest<Array<{
    id: string;
    planCode: string;
    datacenter: string;
    options: string[];
    status: string;
    createdAt: string;
    updatedAt: string;
    retryInterval: number;
    retryCount: number;
    lastCheckTime: number;
  }>>('/api/queue'),
  
  addQueueItem: (item: {
    planCode: string;
    datacenter: string;
    options?: string[];
    retryInterval?: number;
  }) => apiRequest<{ status: string; id: string }>('/api/queue', {
    method: 'POST',
    body: JSON.stringify(item),
  }),
  
  removeQueueItem: (id: string) => apiRequest<{ status: string }>(`/api/queue/${id}`, {
    method: 'DELETE',
  }),
  
  clearQueue: () => apiRequest<{ status: string; count: number }>('/api/queue/clear', {
    method: 'DELETE',
  }),
  
  updateQueueStatus: (id: string, status: string) => 
    apiRequest<{ status: string }>(`/api/queue/${id}/status`, {
      method: 'PUT',
      body: JSON.stringify({ status }),
    }),
  
  // ==================== 购买历史 ====================
  getPurchaseHistory: () => apiRequest<Array<{
    id: string;
    taskId: string;
    planCode: string;
    datacenter: string;
    options: string[];
    status: string;
    orderId: string | null;
    orderUrl: string | null;
    errorMessage: string | null;
    purchaseTime: string;
    attemptCount: number;
    price?: { withTax: number; currency: string };
  }>>('/api/purchase-history'),
  
  clearPurchaseHistory: () => apiRequest<{ status: string }>('/api/purchase-history', {
    method: 'DELETE',
  }),
  
  // ==================== 日志管理 ====================
  getLogs: (limit?: number) => apiRequest<Array<{
    id: string;
    timestamp: string;
    level: string;
    message: string;
    source: string;
  }>>(`/api/logs${limit ? `?limit=${limit}` : ''}`),
  
  clearLogs: () => apiRequest<{ status: string }>('/api/logs', {
    method: 'DELETE',
  }),
  
  flushLogs: () => apiRequest<{ status: string }>('/api/logs/flush', {
    method: 'POST',
  }),
  
  // ==================== 监控系统（专机） ====================
  getSubscriptions: () => apiRequest<Array<{
    planCode: string;
    datacenters: string[];
    notifyAvailable: boolean;
    notifyUnavailable: boolean;
    serverName: string;
    lastStatus: Record<string, string>;
    history: Array<any>;
    autoOrder: boolean;
    quantity: number;
  }>>('/api/monitor/subscriptions'),
  
  addSubscription: (sub: {
    planCode: string;
    datacenters?: string[];
    notifyAvailable?: boolean;
    notifyUnavailable?: boolean;
    autoOrder?: boolean;
    quantity?: number;
  }) => apiRequest<{ status: string; message: string }>('/api/monitor/subscriptions', {
    method: 'POST',
    body: JSON.stringify(sub),
  }),
  
  removeSubscription: (planCode: string) => 
    apiRequest<{ status: string; message: string }>(`/api/monitor/subscriptions/${planCode}`, {
      method: 'DELETE',
    }),
  
  clearSubscriptions: () => 
    apiRequest<{ status: string; count: number }>('/api/monitor/subscriptions/clear', {
      method: 'DELETE',
    }),
  
  batchAddAllServers: (options?: {
    notifyAvailable?: boolean;
    notifyUnavailable?: boolean;
    autoOrder?: boolean;
  }) => apiRequest<{
    status: string;
    added: number;
    skipped: number;
    errors: string[];
    message: string;
  }>('/api/monitor/subscriptions/batch-add-all', {
    method: 'POST',
    body: JSON.stringify(options || {}),
  }),
  
  getSubscriptionHistory: (planCode: string) =>
    apiRequest<{ status: string; planCode: string; history: Array<any> }>(
      `/api/monitor/subscriptions/${planCode}/history`
    ),
  
  startMonitor: () => apiRequest<{ status: string; message: string }>('/api/monitor/start', {
    method: 'POST',
  }),
  
  stopMonitor: () => apiRequest<{ status: string; message: string }>('/api/monitor/stop', {
    method: 'POST',
  }),
  
  getMonitorStatus: () => apiRequest<{
    running: boolean;
    subscriptionCount: number;
    checkInterval: number;
  }>('/api/monitor/status'),
  
  testNotification: () => apiRequest<{ status: string; message: string }>('/api/monitor/test-notification', {
    method: 'POST',
  }),
  
  // ==================== VPS监控 ====================
  getVpsSubscriptions: () => apiRequest<Array<{
    id: string;
    planCode: string;
    datacenters: string[];
    notifyAvailable: boolean;
    notifyUnavailable: boolean;
    lastStatus: Record<string, string>;
    autoOrder: boolean;
  }>>('/api/vps-monitor/subscriptions'),
  
  addVpsSubscription: (sub: {
    planCode: string;
    datacenters?: string[];
    notifyAvailable?: boolean;
    notifyUnavailable?: boolean;
    autoOrder?: boolean;
  }) => apiRequest<{ status: string; message: string }>('/api/vps-monitor/subscriptions', {
    method: 'POST',
    body: JSON.stringify(sub),
  }),
  
  removeVpsSubscription: (id: string) =>
    apiRequest<{ status: string }>(`/api/vps-monitor/subscriptions/${id}`, {
      method: 'DELETE',
    }),
  
  startVpsMonitor: () => apiRequest<{ status: string }>('/api/vps-monitor/start', {
    method: 'POST',
  }),
  
  stopVpsMonitor: () => apiRequest<{ status: string }>('/api/vps-monitor/stop', {
    method: 'POST',
  }),
  
  getVpsMonitorStatus: () => apiRequest<{
    running: boolean;
    subscriptionCount: number;
    checkInterval: number;
  }>('/api/vps-monitor/status'),
  
  // ==================== 配置绑定狙击 ====================
  getConfigSniperTasks: () => apiRequest<Array<{
    id: string;
    api1_planCode: string;
    bound_config: { memory: string; storage: string };
    match_status: string;
    matched_api2: string[];
    enabled: boolean;
    last_check: string | null;
    created_at: string;
  }>>('/api/config-sniper/tasks'),
  
  createConfigSniperTask: (task: {
    api1_planCode: string;
    bound_config: { memory: string; storage: string };
  }) => apiRequest<{
    success: boolean;
    task: any;
    message: string;
  }>('/api/config-sniper/tasks', {
    method: 'POST',
    body: JSON.stringify(task),
  }),
  
  deleteConfigSniperTask: (taskId: string) =>
    apiRequest<{ success: boolean; message: string }>(`/api/config-sniper/tasks/${taskId}`, {
      method: 'DELETE',
    }),
  
  toggleConfigSniperTask: (taskId: string) =>
    apiRequest<{ success: boolean; enabled: boolean }>(`/api/config-sniper/tasks/${taskId}/toggle`, {
      method: 'PUT',
    }),
  
  quickOrder: (order: {
    planCode: string;
    datacenter: string;
    options?: string[];
  }) => apiRequest<{
    success: boolean;
    message: string;
    price?: any;
    options?: string[];
  }>('/api/config-sniper/quick-order', {
    method: 'POST',
    body: JSON.stringify(order),
  }),
  
  // ==================== 价格查询 ====================
  getServerPrice: (planCode: string, datacenter: string, options?: string[]) =>
    apiRequest<{
      success: boolean;
      price?: {
        prices: { withTax: number; withoutTax: number };
        currency: string;
      };
      error?: string;
    }>(`/api/servers/${planCode}/price`, {
      method: 'POST',
      body: JSON.stringify({ datacenter, options: options || [] }),
    }),
  
  // ==================== 服务器控制 ====================
  getMyServers: () => apiRequest<{
    success: boolean;
    servers: Array<{
      serviceName: string;
      name: string;
      commercialRange: string;
      datacenter: string;
      state: string;
      monitoring: boolean;
      reverse: string;
      ip: string;
      os: string;
      status: string;
    }>;
    total: number;
  }>('/api/server-control/list'),
  
  getServerDetails: (serviceName: string) =>
    apiRequest<{
      success: boolean;
      server: any;
    }>(`/api/server-control/${serviceName}`),
  
  rebootServer: (serviceName: string, type?: 'hardware' | 'soft') =>
    apiRequest<{ success: boolean; task: any }>(`/api/server-control/${serviceName}/reboot`, {
      method: 'POST',
      body: JSON.stringify({ type: type || 'soft' }),
    }),
  
  reinstallServer: (serviceName: string, templateName: string, options?: any) =>
    apiRequest<{ success: boolean; message: string; taskId?: number }>(
      `/api/server-control/${serviceName}/install`,
      {
        method: 'POST',
        body: JSON.stringify({ templateName, ...options }),
      }
    ),
  
  getInstallStatus: (serviceName: string) =>
    apiRequest<{
      success: boolean;
      status?: {
        elapsedTime: number;
        progressPercentage: number;
        totalSteps: number;
        completedSteps: number;
        hasError: boolean;
        allDone: boolean;
        steps: Array<{ comment: string; status: string; error: string }>;
      };
      noInstallation?: boolean;
    }>(`/api/server-control/${serviceName}/install/status`),
  
  getServerTasks: (serviceName: string) =>
    apiRequest<{ success: boolean; tasks: Array<any> }>(`/api/server-control/${serviceName}/tasks`),
  
  getServerHardware: (serviceName: string) =>
    apiRequest<{ success: boolean; hardware: any }>(`/api/server-control/${serviceName}/hardware`),
  
  getServerTemplates: (serviceName: string) =>
    apiRequest<{
      success: boolean;
      templates: Array<{ name: string; description: string; category: string }>;
    }>(`/api/server-control/${serviceName}/templates`),
  
  getServerIps: (serviceName: string) =>
    apiRequest<{
      success: boolean;
      ips: Array<{ ip: string; type: string; description: string }>;
    }>(`/api/server-control/${serviceName}/ips`),
  
  getServiceInfo: (serviceName: string) =>
    apiRequest<{
      success: boolean;
      serviceInfo: {
        status: string;
        expiration: string;
        creation: string;
        renewalType: boolean;
        renewalPeriod: number;
      };
    }>(`/api/server-control/${serviceName}/serviceinfo`),
  
  changeContact: (serviceName: string, contacts: {
    contactAdmin?: string;
    contactTech?: string;
    contactBilling?: string;
  }) => apiRequest<{ success: boolean; message: string; taskId?: number }>(
    `/api/server-control/${serviceName}/change-contact`,
    {
      method: 'POST',
      body: JSON.stringify(contacts),
    }
  ),
  
  getInterventions: (serviceName: string) =>
    apiRequest<{ success: boolean; interventions: Array<any> }>(
      `/api/server-control/${serviceName}/interventions`
    ),
  
  getPlannedInterventions: (serviceName: string) =>
    apiRequest<{ success: boolean; plannedInterventions: Array<any> }>(
      `/api/server-control/${serviceName}/planned-interventions`
    ),
  
  // ==================== Telegram ====================
  setTelegramWebhook: (webhookUrl: string) =>
    apiRequest<{
      success: boolean;
      message?: string;
      webhook_url?: string;
      error?: string;
    }>('/api/telegram/set-webhook', {
      method: 'POST',
      body: JSON.stringify({ webhook_url: webhookUrl }),
    }),
  
  getTelegramWebhookInfo: () =>
    apiRequest<{
      success: boolean;
      webhook_info?: any;
      error?: string;
    }>('/api/telegram/get-webhook-info'),
  
  // ==================== 缓存管理 ====================
  getCacheInfo: () => apiRequest<{
    backend: {
      hasCachedData: boolean;
      timestamp: number | null;
      cacheAge: number | null;
      cacheDuration: number;
      serverCount: number;
      cacheValid: boolean;
    };
    storage: {
      dataDir: string;
      cacheDir: string;
      logsDir: string;
      files: Record<string, boolean>;
    };
  }>('/api/cache/info'),
  
  clearCache: (type?: 'all' | 'memory' | 'files') =>
    apiRequest<{
      status: string;
      cleared: string[];
      message: string;
    }>('/api/cache/clear', {
      method: 'POST',
      body: JSON.stringify({ type: type || 'all' }),
    }),
  
  // ==================== OVH账户 ====================
  getOvhAccountInfo: () => apiRequest<{
    success: boolean;
    account?: {
      nichandle: string;
      email: string;
      firstName: string;
      name: string;
      country: string;
    };
    error?: string;
  }>('/api/ovh/account/info'),
  
  getOvhBalance: () => apiRequest<{
    success: boolean;
    balance?: { value: number; currencyCode: string };
    error?: string;
  }>('/api/ovh/account/balance'),
  
  getOvhOrders: (limit?: number) => apiRequest<{
    success: boolean;
    orders?: Array<any>;
    error?: string;
  }>(`/api/ovh/account/orders${limit ? `?limit=${limit}` : ''}`),
  
  getOvhBills: (limit?: number) => apiRequest<{
    success: boolean;
    bills?: Array<any>;
    error?: string;
  }>(`/api/ovh/account/bills${limit ? `?limit=${limit}` : ''}`),
};

export default api;
