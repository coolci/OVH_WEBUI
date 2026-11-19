// Package repository defines data access contracts for persistent storage.
// This decouples business services and handlers from the concrete SQLite implementation.
package repository

import (
	"time"

	"github.com/ovh-webui/server/internal/db"
	"github.com/ovh-webui/server/internal/types"
)

// Pairing code sentinel errors for device repository operations.
var (
	ErrPairingCodeNotFound = db.ErrPairingCodeNotFound
	ErrPairingCodeUsed     = db.ErrPairingCodeUsed
	ErrPairingCodeExpired  = db.ErrPairingCodeExpired

	ParseTelegramButtonOptions = db.ParseTelegramButtonOptions
)

// AccountRepository defines operations for managing configured OVH accounts.
type AccountRepository interface {
	ListAccounts() ([]types.OVHAccount, error)
	GetAccount(id string) (types.OVHAccount, bool, error)
	GetAccountRaw(id string) (db.RawAccount, bool, error)
	CountAccounts() (int, error)
	UpsertAccount(acc types.OVHAccount) error
	DeleteAccount(id string) error
	SetDefaultAccount(id string) error
	UpdateAccountCredState(id string, state string, verifiedAt string, reason string) error
}

// QueueRepository defines operations for managing the automated purchase queue.
type QueueRepository interface {
	ListQueue() ([]types.QueueItem, error)
	ReplaceQueue(items []types.QueueItem) error
	DeleteQueueItem(id string) error
	ClearQueue() (int64, error)
}

// HistoryRepository defines operations for completed/attempted purchase history.
type HistoryRepository interface {
	ListHistory() ([]types.PurchaseHistoryEntry, error)
	ReplaceHistory(items []types.PurchaseHistoryEntry) error
	ClearHistory() (int64, error)
}

// CatalogRepository defines operations for caching public OVH pricing and catalogs.
type CatalogRepository interface {
	GetCatalog(subsidiary string) (string, int64, bool, error)
	UpsertCatalog(subsidiary string, rawJSON string) error
	ClearCatalogs() error
}

// ServerAliasRepository defines operations for local dedicated server nicknames.
type ServerAliasRepository interface {
	ListAliasesByAccount(accountID string) (map[string]string, error)
	UpsertAlias(accountID, serviceName, alias string) error
	DeleteAlias(accountID, serviceName string) error
}

// MonitorRepository defines operations for dedicated server and VPS stock subscriptions.
type MonitorRepository interface {
	ListMonitorSubscriptions() ([]types.Subscription, error)
	UpsertMonitorSubscription(sub types.Subscription) error
	DeleteMonitorSubscription(planCode string) error
	ClearMonitorSubscriptions() (int64, error)
}

// DeviceRepository defines operations for mobile app pairing codes and authorized devices.
type DeviceRepository interface {
	CreatePairingCode() (string, time.Time, error)
	RedeemPairingCode(code, deviceName string) (string, int64, error)
	ListDevices() ([]db.AppDevice, error)
	RevokeDevice(id int64) error
	DeviceTokenValid(token string) (deviceID int64, ok bool)
}

// TelegramRepository defines operations for Telegram idempotency, button clicks, and short IDs.
type TelegramRepository interface {
	FindShortIDByFull(full string) (string, bool, error)
	UpsertShortID(shortID, full, category string) error
	GetShortID(shortID string) (string, bool, error)
	ClaimTelegramButton(buttonID string) (db.TelegramButtonRow, bool, error)
	UnclaimTelegramButton(buttonID string) error
	GetTelegramButton(buttonID string) (db.TelegramButtonRow, bool, error)
	TryClaimTelegramUpdate(updateID int64) (bool, error)
	CleanupTelegramUpdates(beforeUnix float64) (int64, error)
}

// ServerRepository defines server and catalog cache operations.
type ServerRepository interface {
	ServerCount() (int, error)
	ServersUpdatedAt() (int64, error)
	ClearServers() error
	ClearCatalogs() error
}
