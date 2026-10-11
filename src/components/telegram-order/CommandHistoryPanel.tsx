import { History, Trash2, Play } from "lucide-react";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { CommandHistory } from "./types";
import { ORDER_MODES } from "./OrderModeGrid";

interface CommandHistoryPanelProps {
  history: CommandHistory[];
  onClear: () => void;
  onRepeat: (item: CommandHistory) => void;
}

export function CommandHistoryPanel({ history, onClear, onRepeat }: CommandHistoryPanelProps) {
  return (
    <Card className="surface-card rounded-xl border-border flex flex-col">
      <CardHeader className="p-4 sm:p-5 pb-3 border-b border-border/40 flex flex-row items-center justify-between">
        <CardTitle className="text-sm font-semibold flex items-center gap-2">
          <History className="w-4 h-4 text-primary" />
          <span>命令历史</span>
        </CardTitle>
        {history.length > 0 && (
          <Button
            variant="ghost"
            size="sm"
            onClick={onClear}
            className="h-7 px-2.5 text-xs text-muted-foreground hover:text-destructive gap-1.5 rounded-lg"
          >
            <Trash2 className="w-3.5 h-3.5" />
            <span>清空</span>
          </Button>
        )}
      </CardHeader>
      <CardContent className="p-4 sm:p-5 flex-1">
        {history.length === 0 ? (
          <div className="text-center py-10 text-muted-foreground">
            <History className="h-10 w-10 mx-auto mb-2.5 opacity-20" />
            <p className="text-xs">暂无历史记录</p>
          </div>
        ) : (
          <div className="space-y-2 max-h-80 overflow-y-auto pr-1">
            {history.map((item) => (
              <div
                key={item.id}
                className={cn(
                  "p-2.5 rounded-lg border transition-all hover:border-primary/40 cursor-pointer group bg-secondary/20",
                  item.success ? "border-border/70" : "border-destructive/30"
                )}
                onClick={() => onRepeat(item)}
              >
                <div className="flex items-center justify-between gap-2">
                  <code className="font-mono text-xs text-primary truncate">
                    {item.command}
                  </code>
                  <Button
                    variant="ghost"
                    size="sm"
                    className="opacity-0 group-hover:opacity-100 transition-opacity h-6 w-6 p-0 rounded-md"
                  >
                    <Play className="h-3 w-3" />
                  </Button>
                </div>
                <div className="flex items-center gap-2 mt-1.5 flex-wrap">
                  <Badge variant="outline" className="text-[10px] px-1.5 py-0 rounded">
                    {ORDER_MODES.find(m => m.mode === item.mode)?.name || item.mode}
                  </Badge>
                  <span className="text-[11px] text-muted-foreground font-mono">
                    {new Date(item.timestamp).toLocaleString("zh-CN")}
                  </span>
                  {!item.success && (
                    <span className="text-[10px] text-destructive font-medium">失败</span>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
