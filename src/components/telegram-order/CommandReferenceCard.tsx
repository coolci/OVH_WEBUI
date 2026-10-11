import { Info, Copy, Zap } from "lucide-react";
import { toast } from "sonner";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { COMMAND_REFERENCE } from "./types";

export function CommandReferenceCard() {
  return (
    <Card className="surface-card rounded-xl border-border">
      <CardHeader className="p-4 sm:p-5 pb-3 border-b border-border/40">
        <CardTitle className="text-sm font-semibold flex items-center gap-2">
          <Info className="w-4 h-4 text-primary" />
          <span>快捷指令参考与使用说明</span>
        </CardTitle>
      </CardHeader>
      <CardContent className="p-4 sm:p-5 space-y-4">
        {/* Desktop Table View */}
        <div className="hidden md:block overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="text-muted-foreground uppercase border-b border-border/60 text-[11px]">
                <th className="text-left py-2.5 px-3">命令</th>
                <th className="text-left py-2.5 px-3">格式</th>
                <th className="text-left py-2.5 px-3">说明</th>
                <th className="text-left py-2.5 px-3">示例</th>
              </tr>
            </thead>
            <tbody>
              {COMMAND_REFERENCE.map((cmd) => (
                <tr
                  key={cmd.command}
                  className="border-b border-border/40 hover:bg-secondary/30 transition-colors"
                >
                  <td className="py-2.5 px-3">
                    <Badge variant="outline" className={cn("font-mono text-xs px-2 py-0.5 rounded-md", cmd.badgeClass)}>
                      {cmd.command}
                    </Badge>
                  </td>
                  <td className="py-2.5 px-3 font-mono text-muted-foreground">
                    {cmd.format}
                  </td>
                  <td className="py-2.5 px-3 text-foreground/90">
                    {cmd.description}
                  </td>
                  <td className="py-2.5 px-3 font-mono text-primary">
                    <div className="flex items-center gap-1.5">
                      <span>{cmd.example}</span>
                      <button
                        type="button"
                        onClick={() => {
                          navigator.clipboard.writeText(cmd.example);
                          toast.success(`已复制: ${cmd.example}`);
                        }}
                        className="text-muted-foreground hover:text-primary transition-colors p-1 rounded hover:bg-secondary"
                        title="复制示例"
                      >
                        <Copy className="h-3 w-3" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Mobile Command Cards */}
        <div className="md:hidden grid grid-cols-1 gap-2.5">
          {COMMAND_REFERENCE.map((cmd) => (
            <div
              key={cmd.command}
              className="p-3 rounded-xl border border-border/70 bg-secondary/20 space-y-2"
            >
              <div className="flex items-center justify-between">
                <Badge variant="outline" className={cn("font-mono text-xs px-2 py-0.5 rounded-md", cmd.badgeClass)}>
                  {cmd.command}
                </Badge>
                <button
                  type="button"
                  onClick={() => {
                    navigator.clipboard.writeText(cmd.example);
                    toast.success(`已复制: ${cmd.example}`);
                  }}
                  className="inline-flex items-center gap-1 text-[11px] font-mono text-primary bg-primary/10 hover:bg-primary/20 px-2 py-1 rounded-md border border-primary/20 transition-colors"
                  title="复制示例"
                >
                  <span>{cmd.example}</span>
                  <Copy className="h-3 w-3 ml-0.5" />
                </button>
              </div>
              <p className="text-xs text-foreground/90">{cmd.description}</p>
              <div className="text-[11px] font-mono text-muted-foreground bg-secondary/50 px-2.5 py-1 rounded-md">
                格式: {cmd.format}
              </div>
            </div>
          ))}
        </div>

        {/* Free-form & Tips */}
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3 pt-2">
          <div className="p-3.5 bg-secondary/30 rounded-xl border border-border/70 text-xs space-y-2">
            <div className="flex items-center gap-1.5 font-medium text-foreground">
              <Zap className="h-3.5 w-3.5 text-amber-500" />
              <span>免斜杠极速模式 (直接发送)</span>
            </div>
            <p className="text-muted-foreground leading-relaxed text-[12px]">
              在 Telegram 聊天框或快捷命令中直接发送（无需斜杠前缀）：
            </p>
            <div className="flex items-center justify-between bg-background/80 px-3 py-2 rounded-lg border border-border/70 font-mono text-xs text-primary">
              <span>24ska01 gra 1</span>
              <button
                type="button"
                onClick={() => {
                  navigator.clipboard.writeText("24ska01 gra 1");
                  toast.success("已复制: 24ska01 gra 1");
                }}
                className="text-muted-foreground hover:text-primary transition-colors ml-2 p-1 rounded hover:bg-secondary"
                title="复制"
              >
                <Copy className="h-3.5 w-3.5" />
              </button>
            </div>
            <p className="text-muted-foreground text-[11px]">
              格式: <code className="font-mono text-foreground/80 bg-secondary/50 px-1.5 py-0.5 rounded">&lt;型号&gt; [机房] [数量] [系统选项]</code>
            </p>
          </div>

          <div className="p-3.5 bg-secondary/30 rounded-xl border border-border/70 text-xs space-y-2">
            <div className="flex items-center gap-1.5 font-medium text-foreground">
              <Info className="h-3.5 w-3.5 text-primary" />
              <span>使用与安全说明</span>
            </div>
            <ul className="text-muted-foreground space-y-1.5 text-[11px] leading-relaxed list-disc list-inside">
              <li>
                <span className="text-foreground/90 font-medium">鉴权安全：</span>
                仅在【系统设置】中授权的 <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">Chat ID</code> 可下单。
              </li>
              <li>
                <span className="text-foreground/90 font-medium">机房代码：</span>
                支持常用缩写如 <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">gra</code> / <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">rbx</code> / <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">bhs</code> / <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">sbg</code> / <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">waw</code>（不区分大小写）。
              </li>
              <li>
                <span className="text-foreground/90 font-medium">交互点选：</span>
                在 Telegram 中发送 <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">/start</code> 或 <code className="font-mono text-primary bg-secondary/50 px-1 py-0.5 rounded">/buy</code> 亦可展开多级菜单点选。
              </li>
            </ul>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
