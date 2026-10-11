import { MessageSquare, CheckCircle2, Info } from "lucide-react";
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import { cn } from "@/lib/utils";

interface TerminalConsoleProps {
  selectedMode: string;
  lastResult: any;
  currentCommand: string;
}

export function TerminalConsole({ selectedMode, lastResult, currentCommand }: TerminalConsoleProps) {
  return (
    <Card className="surface-card rounded-2xl border-border flex flex-col overflow-hidden shadow-sm">
      <CardHeader className="p-3 sm:p-4 pb-2.5 border-b border-border/40 flex flex-row items-center justify-between bg-secondary/30">
        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1.5 mr-1">
            <span className="w-2.5 h-2.5 rounded-full bg-destructive/60" />
            <span className="w-2.5 h-2.5 rounded-full bg-amber-500/60" />
            <span className="w-2.5 h-2.5 rounded-full bg-emerald-500/60" />
          </div>
          <span className="text-xs font-mono font-medium text-foreground/80">tg-console</span>
        </div>
        <span className="text-[10px] font-mono text-muted-foreground uppercase bg-secondary px-2 py-0.5 rounded border border-border/60">
          {selectedMode}
        </span>
      </CardHeader>
      <CardContent className="p-4 sm:p-5 flex-1 flex flex-col justify-start">
        {lastResult ? (
          <div className="space-y-3 w-full">
            {/* Terminal Input Line */}
            <div className="flex items-center gap-2 text-xs font-mono text-muted-foreground pb-2 border-b border-border/50">
              <span className="text-primary font-bold">›</span>
              <span className="text-foreground font-semibold">{lastResult.command || currentCommand}</span>
            </div>

            <div className={cn(
              "p-3.5 rounded-xl border font-mono text-xs",
              lastResult.success
                ? "bg-primary/5 border-primary/25 text-foreground"
                : "bg-destructive/10 border-destructive/25 text-destructive"
            )}>
              <div className="flex items-center gap-2 mb-2 font-sans font-semibold">
                {lastResult.success ? (
                  <CheckCircle2 className="h-4 w-4 text-primary flex-shrink-0" />
                ) : (
                  <Info className="h-4 w-4 text-destructive flex-shrink-0" />
                )}
                <span>{lastResult.success ? "Bot 响应正常" : "执行异常"}</span>
              </div>
              <p className="whitespace-pre-wrap leading-relaxed text-[11px] sm:text-xs">
                {lastResult.message || lastResult.error}
              </p>
            </div>

            {lastResult.price && (
              <div className="p-3 bg-secondary/30 rounded-xl border border-border/60 flex items-center justify-between">
                <span className="text-xs text-muted-foreground">价格信息</span>
                <span className="text-base font-bold font-mono text-foreground">
                  {lastResult.price.prices?.withoutTax?.toFixed(2) ||
                    lastResult.price.prices?.withTax?.toFixed(2) ||
                    lastResult.price} €
                </span>
              </div>
            )}

            {lastResult.orderId && (
              <div className="p-3 bg-secondary/30 rounded-xl border border-border/60 flex items-center justify-between">
                <span className="text-xs text-muted-foreground">订单 ID</span>
                <span className="font-mono text-xs font-semibold text-primary">{lastResult.orderId}</span>
              </div>
            )}
          </div>
        ) : (
          <div className="text-center py-12 text-muted-foreground font-mono">
            <MessageSquare className="h-10 w-10 mx-auto mb-3 opacity-20" />
            <div className="flex items-center justify-center gap-1.5 text-xs text-muted-foreground/80">
              <span className="text-primary font-bold">›</span>
              <span>等待指令触发</span>
              <span className="inline-block w-1.5 h-3.5 bg-primary/70 cursor-blink ml-0.5" />
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
