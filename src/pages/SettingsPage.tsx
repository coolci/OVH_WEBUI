import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Helmet } from "react-helmet-async";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { 
  Settings, 
  Key,
  Globe,
  MessageSquare,
  Database,
  Shield,
  Save,
  RefreshCw,
  Trash2,
  CheckCircle2,
  XCircle,
  Eye,
  EyeOff,
  Server,
  Wifi,
  WifiOff,
  Loader2
} from "lucide-react";
import { useState, useEffect } from "react";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import api from "@/lib/api";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";

const SettingsPage = () => {
  const [showSecrets, setShowSecrets] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [isConnected, setIsConnected] = useState(false);
  const [isTesting, setIsTesting] = useState(false);
  
  // 后端连接配置（存储在localStorage）
  const [backendConfig, setBackendConfig] = useState({
    backendUrl: localStorage.getItem('backendUrl') || 'http://localhost:19998',
    apiSecretKey: localStorage.getItem('apiSecretKey') || '',
  });
  
  // OVH API配置（从后端获取）
  const [config, setConfig] = useState({
    appKey: "",
    appSecret: "",
    consumerKey: "",
    endpoint: "ovh-eu",
    zone: "IE",
    iam: "go-ovh-ie",
    tgToken: "",
    tgChatId: "",
  });

  const [cacheInfo, setCacheInfo] = useState<{
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
  } | null>(null);

  const [webhookInfo, setWebhookInfo] = useState<any>(null);
  const [isLoadingWebhook, setIsLoadingWebhook] = useState(false);

  // 检测后端连接
  const checkConnection = async () => {
    try {
      await api.health();
      setIsConnected(true);
      return true;
    } catch {
      setIsConnected(false);
      return false;
    }
  };

  // 加载配置
  const loadConfig = async () => {
    setIsLoading(true);
    try {
      const connected = await checkConnection();
      if (connected) {
        const remoteConfig = await api.getConfig();
        setConfig({
          appKey: remoteConfig.appKey || "",
          appSecret: remoteConfig.appSecret || "",
          consumerKey: remoteConfig.consumerKey || "",
          endpoint: remoteConfig.endpoint || "ovh-eu",
          zone: remoteConfig.zone || "IE",
          iam: remoteConfig.iam || "go-ovh-ie",
          tgToken: remoteConfig.tgToken || "",
          tgChatId: remoteConfig.tgChatId || "",
        });
        
        // 加载缓存信息
        try {
          const cache = await api.getCacheInfo();
          setCacheInfo(cache);
        } catch {}

        // 加载Webhook信息
        loadWebhookInfo();
      }
    } catch (err) {
      console.error("Failed to load config:", err);
    } finally {
      setIsLoading(false);
    }
  };

  // 加载Webhook信息
  const loadWebhookInfo = async () => {
    setIsLoadingWebhook(true);
    try {
      const result = await api.getTelegramWebhookInfo();
      if (result.success) {
        setWebhookInfo(result.webhook_info);
      }
    } catch (err) {
      console.error("Failed to load webhook info:", err);
    } finally {
      setIsLoadingWebhook(false);
    }
  };

  useEffect(() => {
    loadConfig();
  }, []);

  // 保存后端连接配置
  const saveBackendConfig = () => {
    localStorage.setItem('backendUrl', backendConfig.backendUrl);
    localStorage.setItem('apiSecretKey', backendConfig.apiSecretKey);
    toast.success("后端连接配置已保存");
    loadConfig();
  };

  // 测试后端连接
  const testConnection = async () => {
    setIsTesting(true);
    try {
      const connected = await checkConnection();
      if (connected) {
        toast.success("后端连接成功");
      } else {
        toast.error("后端连接失败");
      }
    } catch {
      toast.error("后端连接失败");
    } finally {
      setIsTesting(false);
    }
  };

  // 保存OVH配置到后端
  const handleSave = async () => {
    setIsSaving(true);
    try {
      await api.saveConfig(config);
      toast.success("配置已保存");
    } catch (err: any) {
      toast.error(`保存失败: ${err.message}`);
    } finally {
      setIsSaving(false);
    }
  };

  // 验证OVH API连接
  const testOvhApi = async () => {
    try {
      const result = await api.getOvhAccountInfo();
      if (result.success) {
        toast.success(`OVH API 连接成功: ${result.account?.nichandle}`);
      } else {
        toast.error(`OVH API 连接失败: ${result.error}`);
      }
    } catch (err: any) {
      toast.error(`OVH API 连接失败: ${err.message}`);
    }
  };

  // 测试Telegram通知
  const testTelegram = async () => {
    try {
      const result = await api.testNotification();
      toast.success(result.message || "测试消息已发送");
    } catch (err: any) {
      toast.error(`发送失败: ${err.message}`);
    }
  };

  // 设置Telegram Webhook
  const setWebhook = async () => {
    const webhookUrl = prompt("请输入 Webhook URL (包含域名):");
    if (!webhookUrl) return;
    
    try {
      const result = await api.setTelegramWebhook(webhookUrl);
      if (result.success) {
        toast.success("Webhook 设置成功");
      } else {
        toast.error(`设置失败: ${result.error}`);
      }
    } catch (err: any) {
      toast.error(`设置失败: ${err.message}`);
    }
  };

  // 清空缓存
  const clearCache = async (type?: 'all' | 'memory' | 'files') => {
    try {
      const result = await api.clearCache(type);
      toast.success(result.message);
      const cache = await api.getCacheInfo();
      setCacheInfo(cache);
    } catch (err: any) {
      toast.error(`清空失败: ${err.message}`);
    }
  };

  const maskValue = (value: string) => {
    if (!value) return "";
    if (showSecrets) return value;
    if (value.length <= 8) return "••••••••";
    return value.substring(0, 4) + "••••••••" + value.substring(value.length - 4);
  };

  return (
    <>
      <Helmet>
        <title>系统设置 | OVH Sniper</title>
        <meta name="description" content="配置OVH API和系统参数" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              <div>
                <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                  <span className="text-muted-foreground">&gt;</span>
                  系统设置
                  <span className="cursor-blink">_</span>
                </h1>
                <p className="text-sm text-muted-foreground mt-1">
                  配置 API 凭证和系统参数
                </p>
              </div>
              {isConnected ? (
                <span className="flex items-center gap-1 text-xs text-primary bg-primary/10 px-2 py-1 rounded">
                  <Wifi className="h-3 w-3" /> 已连接
                </span>
              ) : (
                <span className="flex items-center gap-1 text-xs text-destructive bg-destructive/10 px-2 py-1 rounded">
                  <WifiOff className="h-3 w-3" /> 未连接
                </span>
              )}
            </div>
            
            <div className="flex gap-2">
              <Button 
                variant="outline" 
                size="sm"
                onClick={() => setShowSecrets(!showSecrets)}
              >
                {showSecrets ? (
                  <><EyeOff className="h-4 w-4 mr-2" />隐藏密钥</>
                ) : (
                  <><Eye className="h-4 w-4 mr-2" />显示密钥</>
                )}
              </Button>
              <Button size="sm" onClick={handleSave} disabled={isSaving || !isConnected}>
                {isSaving ? (
                  <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                ) : (
                  <Save className="h-4 w-4 mr-2" />
                )}
                保存设置
              </Button>
            </div>
          </div>

          <Tabs defaultValue="backend" className="space-y-6">
            <TabsList className="bg-muted/50 border border-border">
              <TabsTrigger value="backend" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <Server className="h-4 w-4 mr-2" />
                后端连接
              </TabsTrigger>
              <TabsTrigger value="ovh" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <Globe className="h-4 w-4 mr-2" />
                OVH API
              </TabsTrigger>
              <TabsTrigger value="telegram" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <MessageSquare className="h-4 w-4 mr-2" />
                Telegram
              </TabsTrigger>
              <TabsTrigger value="security" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <Shield className="h-4 w-4 mr-2" />
                安全
              </TabsTrigger>
              <TabsTrigger value="cache" className="data-[state=active]:bg-primary data-[state=active]:text-primary-foreground">
                <Database className="h-4 w-4 mr-2" />
                缓存
              </TabsTrigger>
            </TabsList>

            {/* Backend Connection Settings */}
            <TabsContent value="backend">
              <TerminalCard
                title="后端连接配置"
                icon={<Server className="h-4 w-4" />}
              >
                <div className="space-y-6">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div className="space-y-2">
                      <Label>后端地址 (Backend URL)</Label>
                      <Input 
                        value={backendConfig.backendUrl}
                        onChange={(e) => setBackendConfig({...backendConfig, backendUrl: e.target.value})}
                        placeholder="http://localhost:19998"
                      />
                      <p className="text-xs text-muted-foreground">
                        Flask 后端服务器地址
                      </p>
                    </div>
                    <div className="space-y-2">
                      <Label>API 密钥 (X-API-Key)</Label>
                      <Input 
                        value={showSecrets ? backendConfig.apiSecretKey : maskValue(backendConfig.apiSecretKey)}
                        onChange={(e) => setBackendConfig({...backendConfig, apiSecretKey: e.target.value})}
                        placeholder="输入 API 密钥"
                        type={showSecrets ? "text" : "password"}
                      />
                      <p className="text-xs text-muted-foreground">
                        需要与后端 .env 文件中的 API_SECRET_KEY 一致
                      </p>
                    </div>
                  </div>
                  
                  <div className="flex gap-2 pt-4 border-t border-border">
                    <Button variant="terminal" onClick={saveBackendConfig}>
                      <Save className="h-4 w-4 mr-2" />
                      保存连接配置
                    </Button>
                    <Button variant="outline" onClick={testConnection} disabled={isTesting}>
                      {isTesting ? (
                        <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                      ) : (
                        <CheckCircle2 className="h-4 w-4 mr-2" />
                      )}
                      测试连接
                    </Button>
                  </div>
                </div>
              </TerminalCard>
            </TabsContent>

            {/* OVH API Settings */}
            <TabsContent value="ovh">
              <TerminalCard
                title="OVH API 配置"
                icon={<Globe className="h-4 w-4" />}
              >
                {!isConnected ? (
                  <div className="text-center py-8 text-muted-foreground">
                    <WifiOff className="h-8 w-8 mx-auto mb-2 opacity-50" />
                    <p>请先配置后端连接</p>
                  </div>
                ) : (
                  <div className="space-y-6">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                      <div className="space-y-2">
                        <Label>Application Key</Label>
                        <Input 
                          value={showSecrets ? config.appKey : maskValue(config.appKey)}
                          onChange={(e) => setConfig({...config, appKey: e.target.value})}
                          placeholder="输入 Application Key"
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>Application Secret</Label>
                        <Input 
                          value={showSecrets ? config.appSecret : maskValue(config.appSecret)}
                          onChange={(e) => setConfig({...config, appSecret: e.target.value})}
                          placeholder="输入 Application Secret"
                          type={showSecrets ? "text" : "password"}
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>Consumer Key</Label>
                        <Input 
                          value={showSecrets ? config.consumerKey : maskValue(config.consumerKey)}
                          onChange={(e) => setConfig({...config, consumerKey: e.target.value})}
                          placeholder="输入 Consumer Key"
                          type={showSecrets ? "text" : "password"}
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>API Endpoint</Label>
                        <Select value={config.endpoint} onValueChange={(v) => setConfig({...config, endpoint: v})}>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="ovh-eu">OVH Europe (ovh-eu)</SelectItem>
                            <SelectItem value="ovh-us">OVH US (ovh-us)</SelectItem>
                            <SelectItem value="ovh-ca">OVH Canada (ovh-ca)</SelectItem>
                          </SelectContent>
                        </Select>
                        <div className="flex gap-1 pt-1">
                          <Button 
                            variant="outline" 
                            size="sm" 
                            className="text-xs"
                            onClick={() => setConfig({...config, endpoint: 'ovh-eu', zone: 'IE'})}
                          >
                            🇪🇺 EU
                          </Button>
                          <Button 
                            variant="outline" 
                            size="sm" 
                            className="text-xs"
                            onClick={() => setConfig({...config, endpoint: 'ovh-us', zone: 'US'})}
                          >
                            🇺🇸 US
                          </Button>
                          <Button 
                            variant="outline" 
                            size="sm" 
                            className="text-xs"
                            onClick={() => setConfig({...config, endpoint: 'ovh-ca', zone: 'CA'})}
                          >
                            🇨🇦 CA
                          </Button>
                        </div>
                      </div>
                      <div className="space-y-2">
                        <Label>Zone / Subsidiary</Label>
                        <Select value={config.zone} onValueChange={(v) => setConfig({...config, zone: v})}>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="IE">爱尔兰 (IE)</SelectItem>
                            <SelectItem value="DE">德国 (DE)</SelectItem>
                            <SelectItem value="FR">法国 (FR)</SelectItem>
                            <SelectItem value="GB">英国 (GB)</SelectItem>
                            <SelectItem value="US">美国 (US)</SelectItem>
                            <SelectItem value="CA">加拿大 (CA)</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                      <div className="space-y-2">
                        <Label>IAM 标识</Label>
                        <Input 
                          value={config.iam}
                          onChange={(e) => setConfig({...config, iam: e.target.value})}
                          placeholder="输入 IAM 标识"
                        />
                      </div>
                    </div>
                    
                    <div className="pt-4 border-t border-border">
                      <Button variant="terminal" onClick={testOvhApi}>
                        <CheckCircle2 className="h-4 w-4 mr-2" />
                        验证 API 连接
                      </Button>
                    </div>
                  </div>
                )}
              </TerminalCard>
            </TabsContent>

            {/* Telegram Settings */}
            <TabsContent value="telegram">
              <TerminalCard
                title="Telegram 通知配置"
                icon={<MessageSquare className="h-4 w-4" />}
              >
                {!isConnected ? (
                  <div className="text-center py-8 text-muted-foreground">
                    <WifiOff className="h-8 w-8 mx-auto mb-2 opacity-50" />
                    <p>请先配置后端连接</p>
                  </div>
                ) : (
                  <div className="space-y-6">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                      <div className="space-y-2">
                        <Label>Bot Token</Label>
                        <Input 
                          value={showSecrets ? config.tgToken : maskValue(config.tgToken)}
                          onChange={(e) => setConfig({...config, tgToken: e.target.value})}
                          placeholder="输入 Telegram Bot Token"
                          type={showSecrets ? "text" : "password"}
                        />
                      </div>
                      <div className="space-y-2">
                        <Label>Chat ID</Label>
                        <Input 
                          value={config.tgChatId}
                          onChange={(e) => setConfig({...config, tgChatId: e.target.value})}
                          placeholder="输入 Chat ID"
                        />
                      </div>
                    </div>

                    <div className="space-y-4 pt-4 border-t border-border">
                      <h3 className="text-sm font-medium">Webhook 配置</h3>
                      
                      {/* Webhook Status */}
                      {isLoadingWebhook ? (
                        <div className="p-3 bg-muted/30 rounded border border-border">
                          <Loader2 className="h-4 w-4 animate-spin" />
                        </div>
                      ) : webhookInfo ? (
                        <div className={cn(
                          "p-4 rounded border",
                          webhookInfo.url ? "border-primary/30 bg-primary/5" : "border-muted"
                        )}>
                          <div className="flex items-center justify-between mb-2">
                            <span className="text-sm font-medium">当前 Webhook</span>
                            {webhookInfo.url ? (
                              <CheckCircle2 className="h-4 w-4 text-primary" />
                            ) : (
                              <XCircle className="h-4 w-4 text-muted-foreground" />
                            )}
                          </div>
                          {webhookInfo.url ? (
                            <div className="space-y-1 text-xs text-muted-foreground">
                              <p>URL: <span className="font-mono text-foreground break-all">{webhookInfo.url}</span></p>
                              {webhookInfo.pending_update_count !== undefined && (
                                <p>待处理更新: <span className="text-foreground">{webhookInfo.pending_update_count}</span></p>
                              )}
                              {webhookInfo.last_error_message && (
                                <p className="text-destructive">错误: {webhookInfo.last_error_message}</p>
                              )}
                            </div>
                          ) : (
                            <p className="text-xs text-muted-foreground">未设置 Webhook</p>
                          )}
                        </div>
                      ) : null}

                      <div className="flex gap-4">
                        <Button variant="terminal" onClick={setWebhook}>
                          设置 Webhook
                        </Button>
                        <Button variant="outline" onClick={() => loadWebhookInfo()}>
                          <RefreshCw className="h-4 w-4 mr-2" />
                          刷新状态
                        </Button>
                        <Button variant="outline" onClick={testTelegram}>
                          发送测试消息
                        </Button>
                      </div>
                    </div>
                  </div>
                )}
              </TerminalCard>
            </TabsContent>

            {/* Security Settings */}
            <TabsContent value="security">
              <TerminalCard
                title="安全设置"
                icon={<Shield className="h-4 w-4" />}
              >
                <div className="space-y-6">
                  <div className="p-4 bg-muted/30 rounded border border-border">
                    <p className="text-sm text-muted-foreground">
                      API 密钥已在"后端连接"标签页中配置。该密钥用于前端与后端的安全通信。
                    </p>
                  </div>

                  <div className="space-y-4 pt-4 border-t border-border">
                    <div className="flex items-center justify-between">
                      <div>
                        <Label>请求时间戳验证</Label>
                        <p className="text-xs text-muted-foreground">
                          拒绝时间偏差超过 5 分钟的请求
                        </p>
                      </div>
                      <Switch defaultChecked />
                    </div>
                    <div className="flex items-center justify-between">
                      <div>
                        <Label>IP 白名单</Label>
                        <p className="text-xs text-muted-foreground">
                          仅允许指定 IP 访问
                        </p>
                      </div>
                      <Switch />
                    </div>
                  </div>
                </div>
              </TerminalCard>
            </TabsContent>

            {/* Cache Settings */}
            <TabsContent value="cache">
              <TerminalCard
                title="缓存管理"
                icon={<Database className="h-4 w-4" />}
              >
                {!isConnected ? (
                  <div className="text-center py-8 text-muted-foreground">
                    <WifiOff className="h-8 w-8 mx-auto mb-2 opacity-50" />
                    <p>请先配置后端连接</p>
                  </div>
                ) : cacheInfo ? (
                  <div className="space-y-6">
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                      <div 
                        className={cn(
                          "p-4 rounded-sm border",
                          cacheInfo.backend.cacheValid ? "border-primary/30" : "border-destructive/30"
                        )}
                      >
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-sm font-medium">服务器缓存</span>
                          {cacheInfo.backend.cacheValid ? (
                            <CheckCircle2 className="h-4 w-4 text-primary" />
                          ) : (
                            <XCircle className="h-4 w-4 text-destructive" />
                          )}
                        </div>
                        <div className="space-y-1 text-xs text-muted-foreground">
                          <p>服务器数: <span className="text-foreground">{cacheInfo.backend.serverCount}</span></p>
                          <p>缓存年龄: <span className="text-foreground">
                            {cacheInfo.backend.cacheAge ? `${Math.round(cacheInfo.backend.cacheAge / 60)} 分钟` : 'N/A'}
                          </span></p>
                          <p>状态: <span className={cacheInfo.backend.cacheValid ? "text-primary" : "text-destructive"}>
                            {cacheInfo.backend.cacheValid ? "有效" : "已过期"}
                          </span></p>
                        </div>
                      </div>
                      
                      {Object.entries(cacheInfo.storage.files).map(([name, exists]) => (
                        <div 
                          key={name}
                          className={cn(
                            "p-4 rounded-sm border",
                            exists ? "border-primary/30" : "border-muted"
                          )}
                        >
                          <div className="flex items-center justify-between mb-2">
                            <span className="text-sm font-medium capitalize">{name}</span>
                            {exists ? (
                              <CheckCircle2 className="h-4 w-4 text-primary" />
                            ) : (
                              <XCircle className="h-4 w-4 text-muted-foreground" />
                            )}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            <p>状态: <span className={exists ? "text-primary" : "text-muted-foreground"}>
                              {exists ? "存在" : "不存在"}
                            </span></p>
                          </div>
                        </div>
                      ))}
                    </div>

                    <div className="flex gap-2 pt-4 border-t border-border">
                      <Button variant="outline" onClick={() => clearCache('memory')}>
                        <RefreshCw className="h-4 w-4 mr-2" />
                        清除内存缓存
                      </Button>
                      <Button variant="destructive" onClick={() => clearCache('all')}>
                        <Trash2 className="h-4 w-4 mr-2" />
                        清空全部缓存
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div className="text-center py-8 text-muted-foreground">
                    <RefreshCw className="h-8 w-8 mx-auto mb-2 opacity-50 animate-spin" />
                    <p>加载中...</p>
                  </div>
                )}
              </TerminalCard>
            </TabsContent>
          </Tabs>
        </div>
      </AppLayout>
    </>
  );
};

export default SettingsPage;
