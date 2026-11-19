package telegram

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/config"
	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/logger"
	"github.com/ovh-webui/server/internal/storage"
)

// update_id 是每个 Bot 各自的序列。换了 Bot Token 再重启时若沿用旧 Bot 的 offset,
// 新 Bot 的消息全部被 Telegram 当成"已确认"丢掉 —— Bot 永远不回话。
func TestPollOffsetIsScopedToBot(t *testing.T) {
	dir := t.TempDir()
	database, err := db.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	st := app.NewState(storage.Paths{DataDir: dir}, config.New(database),
		logger.New(filepath.Join(dir, "t.log"), slog.New(slog.NewTextHandler(io.Discard, nil))), database)

	storeOffset(st, "111:AAA", 900000)
	if got := loadOffset(st, "111:AAA"); got != 900000 {
		t.Fatalf("同一个 Bot 应沿用进度, got %d", got)
	}
	if got := loadOffset(st, "222:BBB"); got != 0 {
		t.Fatalf("换了 Bot 不能沿用旧 Bot 的 offset, got %d", got)
	}
	// 老版本存的裸数字:不知道属于哪个 Bot,从头取(已处理过的由 update_id 幂等表去重)
	if err := database.SetKV(kvPollOffset, int64(900000)); err != nil {
		t.Fatal(err)
	}
	if got := loadOffset(st, "111:AAA"); got != 0 {
		t.Fatalf("旧格式 offset 不能直接用, got %d", got)
	}
}
