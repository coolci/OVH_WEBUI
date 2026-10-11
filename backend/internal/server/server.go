package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/updater"
)

// Server 封装 HTTP 服务器全生命周期管理
type Server struct {
	httpServer          *http.Server
	sqlDB               io.Closer
	state               *app.State
	supervisor          Stopper
	console             *slog.Logger
	addr                string
	enableAuth          bool
	usingDefaultAPIKey  bool
	restartPending      atomic.Bool
	shutdownPending     atomic.Bool
	shutdownDone        chan struct{}
}

// Config 服务器运行配置
type Config struct {
	Addr                string
	Handler             http.Handler
	SqlDB               io.Closer
	State               *app.State
	Supervisor          Stopper
	EnableAuth          bool
	UsingDefaultAPIKey  bool
	Console             *slog.Logger
}

// New 创建应用服务实例
func New(cfg Config) *Server {
	return &Server{
		httpServer:         &http.Server{Addr: cfg.Addr, Handler: cfg.Handler},
		sqlDB:              cfg.SqlDB,
		state:              cfg.State,
		supervisor:         cfg.Supervisor,
		console:            cfg.Console,
		addr:               cfg.Addr,
		enableAuth:         cfg.EnableAuth,
		usingDefaultAPIKey: cfg.UsingDefaultAPIKey,
		shutdownDone:       make(chan struct{}),
	}
}

// ListenWithRetry 绑端口，短暂重试。
// 自更新是 execve: 旧进程的监听 fd 虽然在 Shutdown 里关了，但内核回收与 TIME_WAIT 可能引发重绑冲突。
func ListenWithRetry(addr string, state *app.State) (net.Listener, error) {
	var lastErr error
	for i := 0; i < 10; i++ {
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, nil
		}
		lastErr = err
		if i == 0 && state != nil && state.Logger != nil {
			state.Logger.Warn("端口 "+addr+" 暂时绑不上,重试中: "+err.Error(), "system")
		}
		time.Sleep(300 * time.Millisecond)
	}
	return nil, lastErr
}

// GracefulRestart 自更新完成后调用: 先停止接收新请求并等在途收尾，再关数据库，最后替换进程映像。
func (s *Server) GracefulRestart(exe string) {
	s.restartPending.Store(true)

	if s.state != nil && s.state.Logger != nil {
		s.state.Logger.Info("[更新] 正在优雅关闭以完成重启", "version")
		s.state.Logger.Flush()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(ctx); err != nil && s.console != nil {
		s.console.Warn("shutdown", "err", err)
	}
	if s.sqlDB != nil {
		if err := s.sqlDB.Close(); err != nil && s.console != nil {
			s.console.Warn("close sqlite", "err", err)
		}
	}

	if s.state != nil && s.state.Logger != nil {
		s.state.Logger.Info("[更新] 准备用新二进制替换进程映像: "+exe, "version")
		s.state.Logger.Flush()
	}

	if err := updater.Restart(exe); err != nil {
		if s.state != nil && s.state.Logger != nil {
			s.state.Logger.Error("[更新] 替换进程映像失败: "+err.Error()+"，改用启动新进程的方式", "version")
			s.state.Logger.Flush()
		}
		if serr := updater.Spawn(exe); serr != nil {
			if s.state != nil && s.state.Logger != nil {
				s.state.Logger.Error("[更新] 启动新进程也失败了: "+serr.Error()+"。请手动重启程序", "version")
				s.state.Logger.Flush()
			}
			if s.console != nil {
				s.console.Error("restart", "err", err, "spawn", serr)
			}
			os.Exit(1)
		}
		if s.state != nil && s.state.Logger != nil {
			s.state.Logger.Info("[更新] 新进程已拉起,当前进程退出", "version")
			s.state.Logger.Flush()
		}
		os.Exit(0)
	}
}

// Run 启动服务对外监听并阻塞等待信号或关机事件
func (s *Server) Run() error {
	// 安全告警提示
	host := os.Getenv("LISTEN_HOST")
	if !s.enableAuth {
		s.console.Error("⚠️  API 密钥校验已关闭(ENABLE_API_KEY_AUTH=false):任何人都能调用全部接口,包括下单和重装。仅限本地调试")
	} else if s.usingDefaultAPIKey {
		s.console.Error("⚠️  正在使用默认 API 密钥 123456 —— 请立刻在 .env 里设置 API_SECRET_KEY")
		if host == "" {
			s.console.Error("⚠️  并且监听所有网卡(LISTEN_HOST 为空):同网段任何人都能用默认密钥操作你的 OVH 账户")
		}
	}

	ln, err := ListenWithRetry(s.addr, s.state)
	if err != nil {
		s.console.Error("启动失败:端口没能绑上", "addr", s.addr, "err", err)
		if s.state != nil && s.state.Logger != nil {
			s.state.Logger.Error("启动失败,端口 "+s.addr+" 没能绑上: "+err.Error(), "system")
			s.state.Logger.Flush()
		}
		return err
	}

	if s.state != nil && s.state.Logger != nil {
		s.state.Logger.Info("已监听 "+s.addr+",开始对外服务", "system")
	}
	updater.MarkHealthy(s.state)

	// 捕获系统退出信号 (SIGINT, SIGTERM)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		if s.shutdownPending.Load() {
			select {
			case <-s.shutdownDone:
			case <-time.After(30 * time.Second):
				if s.console != nil {
					s.console.Warn("shutdown", "err", "收尾超过 30 秒,强制退出")
				}
			}
			if s.state != nil && s.state.Logger != nil {
				s.state.Logger.Flush()
			}
			os.Exit(0)
		}

		if s.restartPending.Load() {
			return // 自更新已经在收尾
		}
		s.shutdownPending.Store(true)
		if s.console != nil {
			s.console.Info("shutdown", "signal", sig.String())
		}
		var lg *app.State
		if s.state != nil {
			lg = s.state
		}
		_ = lg
		GracefulShutdown(
			s.httpServer,
			s.sqlDB,
			s.supervisor,
			s.state.Logger,
			s.console,
			sig.String(),
			15*time.Second,
		)
		close(s.shutdownDone)
	}()

	if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
		s.console.Error("server run", "err", err)
		return err
	}

	if s.restartPending.Load() {
		time.Sleep(60 * time.Second)
		if s.state != nil && s.state.Logger != nil {
			s.state.Logger.Error("[更新] 等了 60 秒仍未完成重启,放弃并退出。请手动启动程序", "version")
			s.state.Logger.Flush()
		}
		os.Exit(1)
	}

	return nil
}
