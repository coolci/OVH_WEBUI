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
  EyeOff
} from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/utils";
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
  const [config, setConfig] = useState({
    appKey: "ak-xxxxxxxxxxxx",
    appSecret: "as-xxxxxxxxxxxxxxxxxxxx",
    consumerKey: "ck-xxxxxxxxxxxxxxxxxxxx",
    endpoint: "ovh-eu",
    zone: "IE",
    iam: "go-ovh-ie",
    tgToken: "1234567890:ABCDefghijklmnopqrstuvwxyz",
    tgChatId: "-1001234567890",
    apiKey: "sk-xxxxxxxxxxxxxxxxxxxx",
  });

  const [cacheInfo] = useState({
    serverCache: { valid: true, age: 15, size: "2.4MB" },
    logCache: { valid: true, age: 5, size: "856KB" },
    queueCache: { valid: true, age: 1, size: "124KB" },
  });

  const handleSave = () => {
    setIsSaving(true);
    setTimeout(() => setIsSaving(false), 1500);
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
              <Button size="sm" onClick={handleSave} disabled={isSaving}>
                {isSaving ? (
                  <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                ) : (
                  <Save className="h-4 w-4 mr-2" />
                )}
                保存设置
              </Button>
            </div>
          </div>

          <Tabs defaultValue="ovh" className="space-y-6">
            <TabsList className="bg-muted/50 border border-border">
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

            {/* OVH API Settings */}
            <TabsContent value="ovh">
              <TerminalCard
                title="OVH API 配置"
                icon={<Globe className="h-4 w-4" />}
              >
                <div className="space-y-6">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div className="space-y-2">
                      <Label>Application Key</Label>
                      <Input 
                        value={maskValue(config.appKey)}
                        onChange={(e) => setConfig({...config, appKey: e.target.value})}
                        placeholder="输入 Application Key"
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>Application Secret</Label>
                      <Input 
                        value={maskValue(config.appSecret)}
                        onChange={(e) => setConfig({...config, appSecret: e.target.value})}
                        placeholder="输入 Application Secret"
                        type={showSecrets ? "text" : "password"}
                      />
                    </div>
                    <div className="space-y-2">
                      <Label>Consumer Key</Label>
                      <Input 
                        value={maskValue(config.consumerKey)}
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
                    <Button variant="terminal">
                      <CheckCircle2 className="h-4 w-4 mr-2" />
                      验证 API 连接
                    </Button>
                  </div>
                </div>
              </TerminalCard>
            </TabsContent>

            {/* Telegram Settings */}
            <TabsContent value="telegram">
              <TerminalCard
                title="Telegram 通知配置"
                icon={<MessageSquare className="h-4 w-4" />}
              >
                <div className="space-y-6">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div className="space-y-2">
                      <Label>Bot Token</Label>
                      <Input 
                        value={maskValue(config.tgToken)}
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
                    <div className="flex gap-4">
                      <Button variant="terminal">
                        设置 Webhook
                      </Button>
                      <Button variant="outline">
                        发送测试消息
                      </Button>
                    </div>
                  </div>
                </div>
              </TerminalCard>
            </TabsContent>

            {/* Security Settings */}
            <TabsContent value="security">
              <TerminalCard
                title="安全设置"
                icon={<Shield className="h-4 w-4" />}
              >
                <div className="space-y-6">
                  <div className="space-y-2">
                    <Label>API 访问密钥</Label>
                    <div className="flex gap-2">
                      <Input 
                        value={maskValue(config.apiKey)}
                        onChange={(e) => setConfig({...config, apiKey: e.target.value})}
                        placeholder="输入 API 访问密钥"
                        type={showSecrets ? "text" : "password"}
                        className="flex-1"
                      />
                      <Button variant="outline">
                        <RefreshCw className="h-4 w-4 mr-2" />
                        重新生成
                      </Button>
                    </div>
                    <p className="text-xs text-muted-foreground">
                      此密钥用于保护 API 接口，请妥善保管
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
                <div className="space-y-6">
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    {Object.entries(cacheInfo).map(([key, info]) => (
                      <div 
                        key={key}
                        className={cn(
                          "p-4 rounded-sm border",
                          info.valid ? "border-primary/30" : "border-destructive/30"
                        )}
                      >
                        <div className="flex items-center justify-between mb-2">
                          <span className="text-sm font-medium capitalize">
                            {key.replace("Cache", " 缓存")}
                          </span>
                          {info.valid ? (
                            <CheckCircle2 className="h-4 w-4 text-primary" />
                          ) : (
                            <XCircle className="h-4 w-4 text-destructive" />
                          )}
                        </div>
                        <div className="space-y-1 text-xs text-muted-foreground">
                          <p>大小: <span className="text-foreground">{info.size}</span></p>
                          <p>年龄: <span className="text-foreground">{info.age} 分钟</span></p>
                          <p>状态: <span className={info.valid ? "text-primary" : "text-destructive"}>
                            {info.valid ? "有效" : "已过期"}
                          </span></p>
                        </div>
                      </div>
                    ))}
                  </div>

                  <div className="flex gap-2 pt-4 border-t border-border">
                    <Button variant="outline">
                      <RefreshCw className="h-4 w-4 mr-2" />
                      刷新全部缓存
                    </Button>
                    <Button variant="destructive">
                      <Trash2 className="h-4 w-4 mr-2" />
                      清空全部缓存
                    </Button>
                  </div>
                </div>
              </TerminalCard>
            </TabsContent>
          </Tabs>
        </div>
      </AppLayout>
    </>
  );
};

export default SettingsPage;
