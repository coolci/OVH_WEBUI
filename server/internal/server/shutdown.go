package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/ovh-webui/server/internal/logger"
)

// Stopper 定义可停止服务资源的接口
type Stopper interface {
	Stop()
}

// GracefulShutdown 退出前的收尾: 停止接受新请求 → 停后台循环 → 日志落盘 → 关数据库。
//
// 抽成独立函数是为了能测: 这里真正要保证的是两件**可观察**的事 ——
// 缓冲区里的日志写进了文件、数据库正常关闭。这两件在收到 SIGTERM 时
// 原本一件都不会发生(Go 默认当场终止, defer 不执行)。
//
// 顺序不能反: 先停循环，再关闭在途请求，再 Flush，最后 Close DB。
func GracefulShutdown(
	srv *http.Server,
	sqlDB io.Closer,
	stopper Stopper,
	lg *logger.Logger,
	console *slog.Logger,
	reason string,
	wait time.Duration,
) {
	if lg != nil {
		lg.Info("收到 "+reason+",正在优雅退出", "system")
	}

	// 先停后台循环再关资源: 下单/监控的副作用(OVH 侧订单/通知)先于持久化,
	// 中途被杀 → 重启后重放(重复下单/重复通知)。停循环让它们走到下一个安全点再退出
	if stopper != nil {
		stopper.Stop()
		if lg != nil {
			lg.Info("后台循环已停止(TG 轮询/服务器监控/VPS 监控)", "system")
		}
	}

	// 给在途请求留出收尾时间。抢购链路最长的一步是结账,实测几秒级;
	// 超时也要继续往下走,不能因为一个卡住的请求把整个退出流程拖死。
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	if srv != nil {
		if err := srv.Shutdown(ctx); err != nil {
			if console != nil {
				console.Warn("shutdown", "err", err)
			}
			if lg != nil {
				lg.Warn("优雅退出:仍有请求未在期限内收尾,继续关闭 - "+err.Error(), "system")
			}
		}
	}

	if lg != nil {
		lg.Info("优雅退出:请求已收尾,正在关闭数据库", "system")
		lg.Flush()
	}

	if sqlDB != nil {
		if err := sqlDB.Close(); err != nil {
			if console != nil {
				console.Warn("close sqlite", "err", err)
			}
			if lg != nil {
				lg.Error("关闭数据库失败: "+err.Error(), "system")
				lg.Flush()
			}
		}
	}
}
