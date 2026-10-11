package main

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/handlers"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/router"
	"github.com/ovh-webui/server/internal/secret"
	"github.com/ovh-webui/server/internal/server"
	"github.com/ovh-webui/server/internal/storage"
	"github.com/ovh-webui/server/internal/updater"
	"github.com/ovh-webui/server/internal/worker"
)

func main() {
	envPath := envFilePath()
	clearEmptyEnv(
		"OVH_DB_KEY", "API_SECRET_KEY", "TG_ALLOWED_USER_IDS",
		"CORS_ALLOWED_ORIGINS", "TRUSTED_PROXIES", "OVH_UPDATE_API",
		"LISTEN_HOST", "PORT", "DATA_DIR", "CACHE_DIR", "LOGS_DIR",
	)
	_ = godotenv.Load(envPath)

	level := slog.LevelInfo
	if strings.EqualFold(os.Getenv("DEBUG"), "true") {
		level = slog.LevelDebug
	}
	console := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	paths := storage.DefaultPaths()
	if err := paths.EnsureDirs(); err != nil {
		console.Error("create dirs", "err", err)
		os.Exit(1)
	}

	// 凭据落盘加密初始化
	if err := secret.Init(paths.DataDir, envPath); err != nil {
		console.Warn("凭据加密未启用，数据库里的 OVH 密钥与 Telegram Token 将以明文存储", "err", err)
	} else if secret.KeyWasGenerated() {
		console.Info("已自动生成数据库加密密钥并写入 " + secret.KeySource())
		console.Info("请把它连同配置文件一起备份 —— 丢了这把钥匙，已保存的 OVH 凭据和 Telegram Token 就再也解不开了")
	} else {
		console.Info("凭据加密已启用（密钥来自 " + secret.KeySource() + "）")
	}

	sqliteDB, err := db.Open(paths.DataDir)
	if err != nil {
		console.Error("open sqlite", "err", err)
		os.Exit(1)
	}

	if secret.KeyWasGenerated() && !isTrue(os.Getenv("OVH_DB_KEY_RESET")) {
		if has, herr := sqliteDB.HasEncryptedSecrets(); herr == nil && has {
			console.Error("数据库里有加密过的凭据，但找不到对应的密钥，已停止启动")
			console.Error("这通常是配置文件被覆盖/重置了，或者换机器时只拷了数据库没拷配置")
			console.Error("怎么办：把原来的 " + secret.KeyEnv + " 那一行放回 " + envPath +
				"（或用环境变量传进来）；确实找不回来就设 OVH_DB_KEY_RESET=1 启动，" +
				"那些账户需要重新录入 OVH 凭据")
			sqliteDB.Close()
			os.Exit(1)
		}
	}
	defer sqliteDB.Close()

	if n, merr := sqliteDB.EncryptExistingSecrets(); merr != nil {
		console.Warn("迁移明文凭据时出错（不影响启动）", "err", merr)
	} else if n > 0 {
		console.Info("已把数据库里的明文凭据加密", "账户数", n)
	}

	lg := logger.New(paths.LogFile("app.log.json"), console)
	cfgStore := config.New(sqliteDB)
	state := app.NewState(paths, cfgStore, lg, sqliteDB)
	state.APIKey = os.Getenv("API_SECRET_KEY")
	usingDefaultAPIKey := false
	if state.APIKey == "" {
		state.APIKey = "123456"
		usingDefaultAPIKey = true
	}
	state.Port = os.Getenv("PORT")
	if state.Port == "" {
		state.Port = "19998"
	}
	state.LoadAll()

	if n := secret.DecryptFailures(); n > 0 {
		console.Error("有 " + fmt.Sprintf("%d", n) + " 个加密字段解不开 —— 密钥与数据库对不上。" +
			"常见原因:只恢复了 sniper.db 却没恢复 .dbkey,或换过 " + secret.KeyEnv + "。" +
			"把原来的密钥放回去即可;找不回来的话需要在设置页重新录入 OVH 凭据。")
		state.Logger.Error(fmt.Sprintf("[加密] %d 个字段解密失败:密钥与数据库不匹配", n), "system")
	} else if secret.KeyWasGenerated() && len(state.Accounts) > 0 {
		console.Warn("新生成了加密密钥,但库里已有账户 —— 如果这些账户的凭据是空的,说明原密钥丢了")
	}

	updater.CleanupStale()
	if updater.RollbackIfStale(state) {
		if exe, err := os.Executable(); err == nil {
			console.Warn("已回滚到更新前的版本，正在用它重启")
			_ = sqliteDB.Close()
			if err := updater.Restart(exe); err != nil {
				console.Error("回滚后重启失败", "err", err)
				os.Exit(1)
			}
		}
	}

	// 监控引擎与服务依赖装配
	mon := monitor.New(state)
	handlers.SetMonitorRef(mon)
	mon.LoadFromDB()
	console.Info("监控就绪", "checkInterval", mon.CheckInterval())

	// 后台工作协程统一管理编排
	sup := worker.NewSupervisor(state, mon)
	sup.Start()

	enableAuth := !strings.EqualFold(os.Getenv("ENABLE_API_KEY_AUTH"), "false")

	// 路由引擎解耦装配
	var srv *server.Server
	engine := router.New(router.Config{
		State:      state,
		Monitor:    mon,
		EnableAuth: enableAuth,
		HasUI:      hasUI(),
		WebDistFS:  webDistFS(),
		OnRestart: func(exe string) {
			if srv != nil {
				srv.GracefulRestart(exe)
			}
		},
	}, console)

	host := os.Getenv("LISTEN_HOST")
	addr := host + ":" + state.Port

	srv = server.New(server.Config{
		Addr:               addr,
		Handler:            engine,
		SqlDB:              sqliteDB,
		State:              state,
		Supervisor:         sup,
		EnableAuth:         enableAuth,
		UsingDefaultAPIKey: usingDefaultAPIKey,
		Console:            console,
	})

	console.Info("Listening", "addr", addr, "auth", enableAuth, "ui", hasUI(), "dataDir", paths.DataDir)
	if err := srv.Run(); err != nil {
		os.Exit(1)
	}
}

// gracefulShutdown 兼容保留函数（供 package main 的单元测试调用）
func gracefulShutdown(srv *http.Server, sqlDB io.Closer, lg *logger.Logger, console *slog.Logger, reason string, wait time.Duration) {
	server.GracefulShutdown(srv, sqlDB, nil, lg, console, reason, wait)
}

func envFilePath() string {
	if p := strings.TrimSpace(os.Getenv("OVH_ENV_FILE")); p != "" {
		return p
	}
	return ".env"
}

func isTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func clearEmptyEnv(names ...string) {
	for _, n := range names {
		if v, ok := os.LookupEnv(n); ok && strings.TrimSpace(v) == "" {
			_ = os.Unsetenv(n)
		}
	}
}
