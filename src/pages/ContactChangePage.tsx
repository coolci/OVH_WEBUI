import { AppLayout } from "@/components/layout/AppLayout";
import { TerminalCard } from "@/components/ui/terminal-card";
import { Button } from "@/components/ui/button";
import { Helmet } from "react-helmet-async";
import { 
  UserCog, 
  RefreshCw, 
  Check, 
  X, 
  Mail, 
  Loader2,
  ArrowRight,
  Clock,
  CheckCircle2,
  XCircle,
  AlertCircle
} from "lucide-react";
import { useState } from "react";
import { cn } from "@/lib/utils";
import { useContactChangeRequests } from "@/hooks/useApi";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";

const ContactChangePage = () => {
  const { data, isLoading, refetch } = useContactChangeRequests();
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [processingId, setProcessingId] = useState<number | null>(null);
  const [processingAction, setProcessingAction] = useState<string | null>(null);

  const requests = data?.requests || [];

  const handleRefresh = async () => {
    setIsRefreshing(true);
    try {
      await refetch();
      toast.success("数据刷新成功");
    } catch (error: any) {
      toast.error(`刷新失败: ${error.message}`);
    } finally {
      setIsRefreshing(false);
    }
  };

  const handleAccept = async (id: number) => {
    setProcessingId(id);
    setProcessingAction('accept');
    try {
      const result = await api.acceptContactChange(id);
      if (result.success) {
        toast.success("已接受联系人变更请求");
        refetch();
      } else {
        toast.error(result.error || "操作失败");
      }
    } catch (error: any) {
      toast.error(`操作失败: ${error.message}`);
    } finally {
      setProcessingId(null);
      setProcessingAction(null);
    }
  };

  const handleRefuse = async (id: number) => {
    setProcessingId(id);
    setProcessingAction('refuse');
    try {
      const result = await api.refuseContactChange(id);
      if (result.success) {
        toast.success("已拒绝联系人变更请求");
        refetch();
      } else {
        toast.error(result.error || "操作失败");
      }
    } catch (error: any) {
      toast.error(`操作失败: ${error.message}`);
    } finally {
      setProcessingId(null);
      setProcessingAction(null);
    }
  };

  const handleResend = async (id: number) => {
    setProcessingId(id);
    setProcessingAction('resend');
    try {
      const result = await api.resendContactChangeEmail(id);
      if (result.success) {
        toast.success("确认邮件已重新发送");
      } else {
        toast.error(result.error || "发送失败");
      }
    } catch (error: any) {
      toast.error(`发送失败: ${error.message}`);
    } finally {
      setProcessingId(null);
      setProcessingAction(null);
    }
  };

  const getStatusBadge = (state: string) => {
    switch (state) {
      case 'todo':
        return <Badge variant="outline" className="bg-warning/10 text-warning border-warning/30"><Clock className="h-3 w-3 mr-1" />待处理</Badge>;
      case 'doing':
        return <Badge variant="outline" className="bg-primary/10 text-primary border-primary/30"><Loader2 className="h-3 w-3 mr-1 animate-spin" />处理中</Badge>;
      case 'done':
        return <Badge variant="outline" className="bg-accent/10 text-accent border-accent/30"><CheckCircle2 className="h-3 w-3 mr-1" />已完成</Badge>;
      case 'refused':
        return <Badge variant="outline" className="bg-destructive/10 text-destructive border-destructive/30"><XCircle className="h-3 w-3 mr-1" />已拒绝</Badge>;
      default:
        return <Badge variant="outline"><AlertCircle className="h-3 w-3 mr-1" />{state}</Badge>;
    }
  };

  const getContactTypeBadge = (type: string) => {
    switch (type) {
      case 'admin':
        return <Badge variant="secondary">管理员</Badge>;
      case 'tech':
        return <Badge variant="secondary">技术联系人</Badge>;
      case 'billing':
        return <Badge variant="secondary">账单联系人</Badge>;
      default:
        return <Badge variant="secondary">{type}</Badge>;
    }
  };

  const pendingCount = requests.filter(r => r.state === 'todo' || r.state === 'doing').length;

  return (
    <>
      <Helmet>
        <title>联系人变更 | OVH Sniper</title>
        <meta name="description" content="管理OVH联系人变更请求" />
      </Helmet>
      
      <AppLayout>
        <div className="space-y-6">
          {/* Header */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-bold text-primary flex items-center gap-2">
                <span className="text-muted-foreground">&gt;</span>
                联系人变更管理
                <span className="cursor-blink">_</span>
              </h1>
              <p className="text-sm text-muted-foreground mt-1">
                共 {requests.length} 条记录，{pendingCount} 条待处理
              </p>
            </div>
            
            <Button variant="terminal" onClick={handleRefresh} disabled={isRefreshing}>
              <RefreshCw className={cn("h-4 w-4 mr-2", isRefreshing && "animate-spin")} />
              刷新数据
            </Button>
          </div>

          {/* Status Cards */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="terminal-card p-4">
              <div className="flex items-center gap-2 mb-1 text-muted-foreground">
                <UserCog className="h-4 w-4" />
                <span className="text-xs uppercase">总请求数</span>
              </div>
              <p className="text-lg font-bold">{requests.length}</p>
            </div>
            <div className="terminal-card p-4 border-warning/30">
              <div className="flex items-center gap-2 mb-1 text-warning">
                <Clock className="h-4 w-4" />
                <span className="text-xs uppercase">待处理</span>
              </div>
              <p className="text-lg font-bold text-warning">{pendingCount}</p>
            </div>
            <div className="terminal-card p-4 border-accent/30">
              <div className="flex items-center gap-2 mb-1 text-accent">
                <CheckCircle2 className="h-4 w-4" />
                <span className="text-xs uppercase">已完成</span>
              </div>
              <p className="text-lg font-bold text-accent">
                {requests.filter(r => r.state === 'done').length}
              </p>
            </div>
            <div className="terminal-card p-4 border-destructive/30">
              <div className="flex items-center gap-2 mb-1 text-destructive">
                <XCircle className="h-4 w-4" />
                <span className="text-xs uppercase">已拒绝</span>
              </div>
              <p className="text-lg font-bold text-destructive">
                {requests.filter(r => r.state === 'refused').length}
              </p>
            </div>
          </div>

          {/* Requests List */}
          <TerminalCard
            title="变更请求列表"
            icon={<UserCog className="h-4 w-4" />}
          >
            {isLoading ? (
              <div className="flex items-center justify-center py-12">
                <Loader2 className="h-8 w-8 animate-spin text-primary" />
              </div>
            ) : requests.length === 0 ? (
              <div className="text-center py-12 text-muted-foreground">
                <UserCog className="h-12 w-12 mx-auto mb-4 opacity-30" />
                <p>暂无联系人变更请求</p>
              </div>
            ) : (
              <div className="space-y-4">
                {requests.map((request) => (
                  <div 
                    key={request.id}
                    className={cn(
                      "p-4 rounded-sm border transition-all",
                      request.state === 'todo' || request.state === 'doing'
                        ? "border-warning/30 bg-warning/5"
                        : "border-border"
                    )}
                  >
                    <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
                      <div className="flex-1 space-y-3">
                        <div className="flex items-center gap-3 flex-wrap">
                          <span className="font-bold text-foreground font-mono">
                            {request.serviceDomain}
                          </span>
                          {getStatusBadge(request.state)}
                          {getContactTypeBadge(request.contactType)}
                        </div>
                        
                        <div className="flex items-center gap-2 text-sm text-muted-foreground">
                          <span className="font-mono">{request.fromAccount}</span>
                          <ArrowRight className="h-4 w-4 text-primary" />
                          <span className="font-mono text-primary">{request.toAccount}</span>
                        </div>

                        <div className="flex items-center gap-4 text-xs text-muted-foreground">
                          <span>请求者: {request.askingAccount}</span>
                          <span>
                            请求时间: {new Date(request.dateRequest).toLocaleString("zh-CN")}
                          </span>
                          {request.dateDone && (
                            <span>
                              完成时间: {new Date(request.dateDone).toLocaleString("zh-CN")}
                            </span>
                          )}
                        </div>
                      </div>

                      {(request.state === 'todo' || request.state === 'doing') && (
                        <div className="flex gap-2 flex-wrap">
                          <Button
                            variant="default"
                            size="sm"
                            onClick={() => handleAccept(request.id)}
                            disabled={processingId === request.id}
                          >
                            {processingId === request.id && processingAction === 'accept' ? (
                              <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                            ) : (
                              <Check className="h-4 w-4 mr-1" />
                            )}
                            接受
                          </Button>
                          <Button
                            variant="destructive"
                            size="sm"
                            onClick={() => handleRefuse(request.id)}
                            disabled={processingId === request.id}
                          >
                            {processingId === request.id && processingAction === 'refuse' ? (
                              <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                            ) : (
                              <X className="h-4 w-4 mr-1" />
                            )}
                            拒绝
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleResend(request.id)}
                            disabled={processingId === request.id}
                          >
                            {processingId === request.id && processingAction === 'resend' ? (
                              <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                            ) : (
                              <Mail className="h-4 w-4 mr-1" />
                            )}
                            重发邮件
                          </Button>
                        </div>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </TerminalCard>
        </div>
      </AppLayout>
    </>
  );
};

export default ContactChangePage;
