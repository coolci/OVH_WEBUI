import React, { Component, type ErrorInfo, type ReactNode } from "react";
import { AlertTriangle, RefreshCw, Home, ChevronDown } from "lucide-react";
import { Button } from "@/components/ui/button";

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
  showDetails: boolean;
}

/**
 * 全站顶层与业务板块错误边界 (ErrorBoundary)
 * 阻断未捕获异常导致整页白屏，提供快速恢复通道与 Obsidian Slate 诊断卡片。
 */
export class ErrorBoundary extends Component<Props, State> {
  public state: State = {
    hasError: false,
    error: null,
    showDetails: false,
  };

  public static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error, showDetails: false };
  }

  public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("[OVH_WEBUI] ErrorBoundary 捕获到运行时异常:", error, errorInfo);
  }

  private handleReload = () => {
    window.location.reload();
  };

  private handleGoHome = () => {
    window.location.href = "/";
  };

  public render() {
    if (this.state.hasError) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return (
        <div className="min-h-[50vh] flex items-center justify-center p-4 sm:p-6">
          <div className="w-full max-w-lg surface-card rounded-2xl border border-destructive/30 bg-card/90 backdrop-blur-xl p-6 shadow-2xl space-y-5 animate-in fade-in zoom-in-95 duration-200">
            {/* 顶栏警示 */}
            <div className="flex items-start gap-4">
              <div className="w-11 h-11 rounded-xl bg-destructive/15 border border-destructive/30 text-destructive flex items-center justify-center shrink-0 shadow-sm">
                <AlertTriangle className="w-5 h-5" />
              </div>
              <div className="flex-1 min-w-0">
                <h2 className="text-base font-bold text-foreground tracking-tight">
                  页面渲染异常
                </h2>
                <p className="mt-1 text-xs text-muted-foreground leading-relaxed">
                  系统已捕获运行时错误并阻止崩溃白屏。您可以尝试重新加载或返回首页。
                </p>
              </div>
            </div>

            {/* 错误详情展开 */}
            {this.state.error && (
              <div className="space-y-2">
                <button
                  type="button"
                  onClick={() => this.setState((s) => ({ showDetails: !s.showDetails }))}
                  className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors font-medium"
                >
                  <ChevronDown
                    className={`w-3.5 h-3.5 transition-transform duration-150 ${
                      this.state.showDetails ? "rotate-180" : ""
                    }`}
                  />
                  <span>{this.state.showDetails ? "隐藏诊断信息" : "查看错误详情"}</span>
                </button>

                {this.state.showDetails && (
                  <div className="p-3 rounded-xl bg-background/80 border border-border/70 font-mono text-[11px] text-destructive overflow-x-auto max-h-40 leading-relaxed shadow-inner">
                    <p className="font-semibold">{this.state.error.name}: {this.state.error.message}</p>
                    {this.state.error.stack && (
                      <pre className="mt-2 text-[10px] text-muted-foreground whitespace-pre-wrap">
                        {this.state.error.stack.split("\n").slice(0, 5).join("\n")}
                      </pre>
                    )}
                  </div>
                )}
              </div>
            )}

            {/* 操作按钮区 (对齐快速下单实心风格) */}
            <div className="flex items-center justify-end gap-2.5 pt-2 border-t border-border/50">
              <Button
                variant="outline"
                size="sm"
                className="h-8 gap-1.5 text-xs font-medium rounded-lg px-3"
                onClick={this.handleGoHome}
              >
                <Home className="w-3.5 h-3.5" />
                <span>返回首页</span>
              </Button>
              <Button
                size="sm"
                className="h-8 gap-1.5 text-xs font-medium rounded-lg shadow-sm px-3"
                onClick={this.handleReload}
              >
                <RefreshCw className="w-3.5 h-3.5" />
                <span>刷新重试</span>
              </Button>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
