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
  }>('/api/settings'),
  
  saveConfig: (config: {
    appKey: string;
    appSecret: string;
    consumerKey: string;
    endpoint: string;
    tgToken: string;
    tgChatId: string;
    iam: string;
    zone: string;
  }) => apiRequest<{ status: string }>('/api/settings', {
    method: 'POST',
    body: JSON.stringify(config),
  }),
  
  // ==================== 服务器列表 ====================
    getServers: async () => {
      const result = await apiRequest<any>('/api/servers?showApiServers=true');
      const servers = Array.isArray(result)
        ? result
        : Array.isArray(result?.servers)
          ? result.servers
          : [];
      return servers.map((server: any) => ({
        ...server,
        ram: server.ram ?? server.memory ?? "N/A",
        memory: server.memory ?? server.ram ?? "N/A",
      })) as Array<{
        planCode: string;
        name: string;
        cpu: string;
        ram: string;
        memory: string;
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
      }>;
    },
  
    refreshServers: async (force?: boolean) => {
      const result = await apiRequest<any>(
        `/api/servers?forceRefresh=${force ? 'true' : 'false'}&showApiServers=true`
      );
      const servers = Array.isArray(result?.servers)
        ? result.servers
        : Array.isArray(result)
          ? result
          : [];
      const normalizedServers = servers.map((server: any) => ({
        ...server,
        ram: server.ram ?? server.memory ?? "N/A",
        memory: server.memory ?? server.ram ?? "N/A",
      }));
      return { status: "success", count: normalizedServers.length, servers: normalizedServers };
    },

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
  
  // 队列处理器控制
  startQueueProcessor: () => apiRequest<{ status: string; message: string }>('/api/queue/processor/start', {
    method: 'POST',
  }),
  
  stopQueueProcessor: () => apiRequest<{ status: string; message: string }>('/api/queue/processor/stop', {
    method: 'POST',
  }),
  
  getQueueProcessorStatus: () => apiRequest<{
    running: boolean;
    processedCount: number;
    lastProcessTime: number | null;
  }>('/api/queue/processor/status'),
  
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
  
  // 更新监控间隔
  updateMonitorInterval: (interval: number) => apiRequest<{
    status: string;
    checkInterval?: number;
    message?: string;
  }>('/api/monitor/interval', {
    method: 'PUT',
    body: JSON.stringify({ interval }),
  }),
  
  testNotification: () => apiRequest<{ status: string; message: string }>('/api/monitor/test-notification', {
    method: 'POST',
  }),
  
  // 手动检查单个planCode
  manualCheckDedicated: (planCode: string) =>
    apiRequest<any>(`/api/availability/${planCode}`, {
      method: 'POST',
    }),

  getAvailability: (planCode: string, options?: string[]) =>
    apiRequest<Record<string, string>>(`/api/availability/${planCode}`, {
      method: 'POST',
      body: JSON.stringify({ options: options || [] }),
    }),
  
  // ==================== VPS监控 ====================
  getVpsSubscriptions: () => apiRequest<Array<{
    id: string;
    planCode: string;
    displayName?: string;
    ovhSubsidiary?: string;
    datacenters: string[];
    monitorLinux?: boolean;
    monitorWindows?: boolean;
    notifyAvailable: boolean;
    notifyUnavailable: boolean;
    lastStatus: Record<string, { linux?: string; windows?: string } | string>;
    autoOrder: boolean;
    createdAt?: string;
  }>>('/api/vps-monitor/subscriptions'),
  
  addVpsSubscription: (sub: {
    planCode: string;
    ovhSubsidiary?: string;
    datacenters?: string[];
    monitorLinux?: boolean;
    monitorWindows?: boolean;
    notifyAvailable?: boolean;
    notifyUnavailable?: boolean;
    autoOrder?: boolean;
  }) => apiRequest<{ status: string; message: string; id?: string }>('/api/vps-monitor/subscriptions', {
    method: 'POST',
    body: JSON.stringify(sub),
  }),
  
  removeVpsSubscription: (id: string) =>
    apiRequest<{ status: string }>(`/api/vps-monitor/subscriptions/${id}`, {
      method: 'DELETE',
    }),
  
  clearVpsSubscriptions: () =>
    apiRequest<{ status: string; count: number }>('/api/vps-monitor/subscriptions/clear', {
      method: 'DELETE',
    }),
  
  batchAddAllVps: (options?: {
    notifyAvailable?: boolean;
    notifyUnavailable?: boolean;
    autoOrder?: boolean;
  }) => apiRequest<{
    status: string;
    added: number;
    skipped: number;
    errors: string[];
    message: string;
  }>('/api/vps-monitor/subscriptions/batch-add-all', {
    method: 'POST',
    body: JSON.stringify(options || {}),
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
  
  // VPS监控历史
  getVpsSubscriptionHistory: (id: string) =>
    apiRequest<{ status: string; id: string; history: Array<any> }>(
      `/api/vps-monitor/subscriptions/${id}/history`
    ),
  
  // 更新VPS监控间隔
  updateVpsMonitorInterval: (interval: number) => apiRequest<{
    status: string;
    checkInterval?: number;
    message?: string;
  }>('/api/vps-monitor/interval', {
    method: 'PUT',
    body: JSON.stringify({ interval }),
  }),
  
  // 手动检查VPS可用性
  manualCheckVps: (planCode: string, ovhSubsidiary?: string) => apiRequest<{
    status: string;
    data?: {
      planCode: string;
      datacenters: Array<{
        datacenter: string;
        code: string;
        status: string;
        daysBeforeDelivery: number;
      }>;
    };
  }>(`/api/vps-monitor/check/${planCode}`, {
    method: 'POST',
    body: JSON.stringify({ ovhSubsidiary: ovhSubsidiary || 'IE' }),
  }),
  
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
  
  getServerDetails: async (serviceName: string) => {
    const result = await apiRequest<any>(`/api/server-control/${serviceName}/serviceinfo`);
    if (result?.success && result?.serviceInfo) {
      return { success: true, server: result.serviceInfo };
    }
    return result;
  },
  
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
  
  getInstallStatus: async (serviceName: string) => {
    const result = await apiRequest<any>(`/api/server-control/${serviceName}/install/status`);
    if (!result?.success) {
      return result;
    }
    if (result?.status) {
      return result;
    }
    if (result?.hasInstallation === false || result?.noInstallation === true) {
      return {
        ...result,
        status: {
          noInstallation: true,
        },
      };
    }
    return result;
  },
  
  getServerTasks: (serviceName: string) =>
    apiRequest<{ success: boolean; tasks: Array<any> }>(`/api/server-control/${serviceName}/tasks`),
  
  getServerHardware: (serviceName: string) =>
    apiRequest<{ success: boolean; hardware: any }>(`/api/server-control/${serviceName}/hardware`),
  
  getServerTemplates: async (serviceName: string) => {
    const result = await apiRequest<any>(`/api/server-control/${serviceName}/templates`);
    if (!result?.success) {
      return result;
    }
    const templates = Array.isArray(result?.templates) ? result.templates : [];
    const normalized: Array<{ name: string; description: string; category: string }> = [];
    for (const template of templates) {
      const name = template?.name ?? template?.templateName ?? "";
      if (!name) continue;
      normalized.push({
        name,
        description: template?.description ?? template?.distribution ?? "",
        category: template?.category ?? template?.family ?? "",
      });
    }
    return { ...result, templates: normalized };
  },
  
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
  
  // ==================== 高级服务器控制 ====================
  // IPMI控制台
  getIpmiAccess: async (serviceName: string) => {
    const result = await apiRequest<any>(`/api/server-control/${serviceName}/console`);
    if (result?.success && result?.console) {
      const consoleUrl =
        result.console.url ||
        result.console.kvmUrl ||
        result.console.value ||
        (typeof result.console === "string" ? result.console : undefined);
      const expires = result.console.expires || result.console.expiration;
      return {
        ...result,
        ipmiInfos: consoleUrl ? { url: consoleUrl, expires } : undefined,
      };
    }
    return result;
  },
  
  // Burst带宽管理
  getBurstStatus: async (serviceName: string) => {
    const result = await apiRequest<any>(`/api/server-control/${serviceName}/burst`);
    if (!result?.success) {
      return result;
    }
    const burst = result?.burst || {};
    const status = typeof burst.status === "string" ? burst.status : undefined;
    const enabled =
      typeof burst.enabled === "boolean"
        ? burst.enabled
        : status
          ? status.toLowerCase() === "active"
          : false;
    return {
      ...result,
      burst: {
        ...burst,
        enabled,
      },
    };
  },
  
  toggleBurst: (serviceName: string, enable: boolean) =>
    apiRequest<{ success: boolean; message?: string; error?: string }>(
      `/api/server-control/${serviceName}/burst`,
      {
        method: 'PUT',
        body: JSON.stringify({ status: enable ? 'active' : 'inactive' }),
      }
    ),
  
  // 防火墙管理
  getFirewallStatus: (serviceName: string) =>
    apiRequest<{
      success: boolean;
      firewall?: {
        enabled: boolean;
        mode: string;
        rules?: Array<any>;
      };
      error?: string;
    }>(`/api/server-control/${serviceName}/firewall`),
  
  toggleFirewall: (serviceName: string, enable: boolean) =>
    apiRequest<{ success: boolean; message?: string; error?: string }>(
      `/api/server-control/${serviceName}/firewall`,
      {
        method: 'PUT',
        body: JSON.stringify({ enabled: enable }),
      }
    ),
  
  // 备份FTP
  getBackupFtp: (serviceName: string) =>
    apiRequest<{
      success: boolean;
      backupFtp?: {
        ftpBackupName: string;
        type: string;
        quota: number;
        usage: number;
        readOnly: boolean;
      };
      error?: string;
    }>(`/api/server-control/${serviceName}/backup-ftp`),
  
  activateBackupFtp: (serviceName: string) =>
    apiRequest<{ success: boolean; message?: string; error?: string }>(
      `/api/server-control/${serviceName}/backup-ftp`,
      {
        method: 'POST',
      }
    ),
  
  getBackupFtpPassword: async (serviceName: string) => {
    const result = await apiRequest<any>(`/api/server-control/${serviceName}/backup-ftp/password`, {
      method: 'POST',
    });
    if (!result?.success) {
      return result;
    }
    return {
      ...result,
      message: result?.message || result?.result?.message,
      password: result?.password,
    } as { success: boolean; password?: string; message?: string; error?: string };
  },
  
  // BIOS设置
  getBiosSettings: (serviceName: string) =>
    apiRequest<{
      success: boolean;
      biosSettings?: {
        supportBiosSettings: boolean;
      };
      error?: string;
    }>(`/api/server-control/${serviceName}/bios-settings`),
  
  // IP迁移
  moveIp: (serviceName: string, ip: string, targetService: string) =>
    apiRequest<{ success: boolean; task?: any; error?: string }>(
      `/api/server-control/${serviceName}/ip/move`,
      {
        method: 'POST',
        body: JSON.stringify({ ip, to: targetService }),
      }
    ),
  
  // 服务终止
  terminateServer: (serviceName: string) =>
    apiRequest<{ success: boolean; message?: string; error?: string }>(
      `/api/server-control/${serviceName}/terminate`,
      {
        method: 'POST',
      }
    ),
  
  confirmTermination: (serviceName: string, token: string) =>
    apiRequest<{ success: boolean; message?: string; error?: string }>(
      `/api/server-control/${serviceName}/confirm-termination`,
      {
        method: 'POST',
        body: JSON.stringify({ token }),
      }
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
  getOvhAccountInfo: async () => {
    const result = await apiRequest<any>('/api/ovh/account/info');
    if (result?.status !== "success") {
      throw new Error(result?.message || result?.error || "Failed to load account info");
    }
    return { success: true, account: result.data };
  },
  
  getOvhBalance: async () => {
    const result = await apiRequest<any>('/api/ovh/account/balance');
    if (result?.status !== "success") {
      throw new Error(result?.message || result?.error || "Failed to load balance");
    }
    return { success: true, balance: result.data };
  },
  
  // 获取信用余额
  getOvhCreditBalance: async () => {
    const result = await apiRequest<any>('/api/ovh/account/credit-balance');
    if (result?.status !== "success") {
      throw new Error(result?.message || result?.error || "Failed to load credit balance");
    }
    return { success: true, data: result.data || [] };
  },
  
  // 获取子账户列表
  getOvhSubAccounts: async () => {
    const result = await apiRequest<any>('/api/ovh/account/sub-accounts');
    if (result?.status !== "success") {
      throw new Error(result?.message || result?.error || "Failed to load sub accounts");
    }
    return { success: true, data: result.data || [] };
  },
  
  getOvhOrders: async (limit?: number) => {
    const result = await apiRequest<any>(`/api/ovh/account/orders${limit ? `?limit=${limit}` : ''}`);
    if (result?.status !== "success") {
      throw new Error(result?.message || result?.error || "Failed to load orders");
    }
    return { success: true, orders: result.data || [] };
  },
  
  getOvhBills: async (limit?: number) => {
    const result = await apiRequest<any>(`/api/ovh/account/bills${limit ? `?limit=${limit}` : ''}`);
    if (result?.status !== "success") {
      throw new Error(result?.message || result?.error || "Failed to load bills");
    }
    return { success: true, bills: result.data || [] };
  },
  
  // ==================== 邮件历史 ====================
  getOvhEmails: async (limit?: number) => {
    const result = await apiRequest<any>(
      `/api/ovh/account/email-history${limit ? `?limit=${limit}` : ''}`
    );
    if (result?.status !== "success") {
      throw new Error(result?.message || result?.error || "Failed to load emails");
    }
    return { success: true, emails: result.data || [] };
  },
  
  // ==================== 退款记录 ====================
  getOvhRefunds: async (limit?: number) => {
    const result = await apiRequest<any>(`/api/ovh/account/refunds${limit ? `?limit=${limit}` : ''}`);
    if (result?.status !== "success") {
      throw new Error(result?.message || result?.error || "Failed to load refunds");
    }
    return { success: true, refunds: result.data || [] };
  },
  
  // ==================== 联系人变更请求 ====================
  getContactChangeRequests: async () => {
    const result = await apiRequest<any>('/api/ovh/contact-change-requests');
    if (result?.status !== "success") {
      throw new Error(result?.message || result?.error || "Failed to load contact change requests");
    }
    return { success: true, requests: result.data || [] };
  },
  
  acceptContactChange: (id: number) =>
    apiRequest<{ success: boolean; message?: string; error?: string }>(
      `/api/ovh/contact-change-requests/${id}/accept`,
      { method: 'POST' }
    ),
  
  refuseContactChange: (id: number) =>
    apiRequest<{ success: boolean; message?: string; error?: string }>(
      `/api/ovh/contact-change-requests/${id}/refuse`,
      { method: 'POST' }
    ),
  
  resendContactChangeEmail: (id: number) =>
    apiRequest<{ success: boolean; message?: string; error?: string }>(
      `/api/ovh/contact-change-requests/${id}/resend-email`,
      { method: 'POST' }
    ),
  
  // ==================== 性能监控 ====================
  getServerStatistics: async (
    serviceName: string,
    period?: 'hourly' | 'daily' | 'weekly' | 'monthly' | 'yearly',
    type?: 'traffic:download' | 'traffic:upload'
  ) => {
    const params = new URLSearchParams();
    if (period) params.set('period', period);
    if (type) params.set('type', type);
    const query = params.toString();

    const result = await apiRequest<any>(
      `/api/server-control/${serviceName}/mrtg${query ? `?${query}` : ''}`
    );

    if (!result?.success) {
      return result;
    }

    let data = Array.isArray(result?.statistics) ? result.statistics : null;
    if (!data && Array.isArray(result?.data)) {
      data = result.data;
    }
    if (!data && Array.isArray(result?.interfaces)) {
      const iface = result.interfaces.find((item: any) => Array.isArray(item?.data) && item.data.length);
      data = iface?.data || [];
    }

    const statistics = Array.isArray(data)
      ? data.map((item: any) => {
          if (Array.isArray(item)) {
            return { timestamp: item[0], value: item[1] };
          }
          return {
            timestamp: item?.timestamp ?? item?.time ?? 0,
            value: item?.value ?? item?.avg ?? 0,
          };
        })
      : [];

    return {
      success: true,
      statistics,
    };
  },
  
  // ==================== Telegram WEBHOOK下单 ====================
  telegramQuickOrder: (order: {
    mode: 'stock' | 'queue' | 'monitor' | 'price' | 'buy';
    planCode?: string;
    datacenter?: string;
    options?: string[];
    quantity?: number;
  }) => apiRequest<{
    success: boolean;
    message?: string;
    orderId?: string;
    price?: any;
    error?: string;
  }>('/api/telegram/quick-order', {
    method: 'POST',
    body: JSON.stringify(order),
  }),
  
  getTelegramOrderModes: () => apiRequest<{
    success: boolean;
    modes: Array<{
      mode: string;
      description: string;
      example: string;
    }>;
  }>('/api/telegram/order-modes'),
  
  // ==================== 更新订阅（含数量） ====================
  updateSubscription: (planCode: string, options: {
    notifyAvailable?: boolean;
    notifyUnavailable?: boolean;
    autoOrder?: boolean;
    quantity?: number;
    datacenters?: string[];
  }) => apiRequest<{
    status: string;
    message?: string;
  }>(`/api/monitor/subscriptions`, {
    method: 'POST',
    body: JSON.stringify({ planCode, ...options }),
  }),
  
  // ==================== 更新VPS订阅 ====================
  updateVpsSubscription: (id: string, options: {
    notifyAvailable?: boolean;
    notifyUnavailable?: boolean;
    autoOrder?: boolean;
    datacenters?: string[];
    monitorLinux?: boolean;
    monitorWindows?: boolean;
  }) => apiRequest<{
    success: boolean;
    message?: string;
    error?: string;
  }>(`/api/vps-monitor/subscriptions/${id}`, {
    method: 'PUT',
    body: JSON.stringify(options),
  }),
};

export default api;
