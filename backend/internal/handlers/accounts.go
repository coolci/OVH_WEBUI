package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// ── 输入 / 输出 DTO ────────────────────────────────────────────────────────

// accountInput POST/PUT body
type accountInput struct {
	Name        string `json:"name"`
	Endpoint    string `json:"endpoint"` // 可空,会按 zone 推断
	Zone        string `json:"zone"`
	AppKey      string `json:"appKey"`
	AppSecret   string `json:"appSecret"`
	ConsumerKey string `json:"consumerKey"`
	IAM         string `json:"iam"`      // 可空,会自动生成 go-ovh-<zone>
	SetDefault  bool   `json:"setDefault"`
}

// endpointForZone 根据 zone 推 endpoint
func endpointForZone(zone string) string {
	switch strings.ToUpper(zone) {
	case "US":
		return "ovh-us"
	case "CA", "QC", "ASIA", "SG", "AU", "IN":
		return "ovh-ca"
	default:
		return "ovh-eu"
	}
}

// fillDerived 补全 Endpoint / IAM
func (in *accountInput) normalize() {
	in.Name = strings.TrimSpace(in.Name)
	in.Zone = strings.ToUpper(strings.TrimSpace(in.Zone))
	in.AppKey = strings.TrimSpace(in.AppKey)
	in.AppSecret = strings.TrimSpace(in.AppSecret)
	in.ConsumerKey = strings.TrimSpace(in.ConsumerKey)
	in.IAM = strings.TrimSpace(in.IAM)
	if in.Zone == "" {
		in.Zone = "IE"
	}
	if in.Endpoint == "" {
		in.Endpoint = endpointForZone(in.Zone)
	}
	if in.IAM == "" {
		in.IAM = "go-ovh-" + strings.ToLower(in.Zone)
	}
}

func (in *accountInput) validate() string {
	if in.Name == "" {
		return "缺少 name"
	}
	if in.AppKey == "" || in.AppSecret == "" || in.ConsumerKey == "" {
		return "缺少 OVH 凭据 (appKey / appSecret / consumerKey)"
	}
	return ""
}

// ── handlers ───────────────────────────────────────────────────────────────

// ListAccounts GET /api/accounts
func ListAccounts(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		accs, err := state.DB.ListAccounts()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if accs == nil {
			accs = []types.OVHAccount{}
		}
		c.JSON(http.StatusOK, gin.H{"accounts": accs, "total": len(accs)})
	}
}

// GetAccountByID GET /api/accounts/:id
func GetAccountByID(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		acc, ok, err := state.DB.GetAccount(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "账户不存在"})
			return
		}
		c.JSON(http.StatusOK, acc)
	}
}

// CreateAccount POST /api/accounts
// 创建后立即用新凭据调 OVH /me 验证,验证失败回滚不入库。
func CreateAccount(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in accountInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		in.normalize()
		if msg := in.validate(); msg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}

		// 没账户时第一个自动设默认
		count, _ := state.DB.CountAccounts()
		isDefault := in.SetDefault || count == 0

		acc := types.OVHAccount{
			ID:          uuid.NewString(),
			Name:        in.Name,
			Endpoint:    in.Endpoint,
			Zone:        in.Zone,
			AppKey:      in.AppKey,
			AppSecret:   in.AppSecret,
			ConsumerKey: in.ConsumerKey,
			IAM:         in.IAM,
			IsDefault:   isDefault,
			CreatedAt:   types.NowISO(),
		}
		if err := state.DB.UpsertAccount(acc); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		_ = state.ReloadAccounts()

		// 用新凭据验证
		valid := verifyAccountCreds(state, acc.ID)
		state.Logger.Info("创建账户: "+acc.Name+" ("+acc.Zone+") valid="+boolStr(valid), "accounts")

		c.JSON(http.StatusOK, gin.H{"account": acc, "valid": valid})
	}
}

// UpdateAccount PUT /api/accounts/:id
func UpdateAccount(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		existing, ok, err := state.DB.GetAccount(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "账户不存在"})
			return
		}
		var in accountInput
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		in.normalize()
		// 允许部分更新:空字段保留原值
		acc := existing
		if in.Name != "" {
			acc.Name = in.Name
		}
		if in.Zone != "" {
			acc.Zone = in.Zone
			acc.Endpoint = in.Endpoint // 跟着 zone 走
			acc.IAM = in.IAM
		}
		if in.AppKey != "" {
			acc.AppKey = in.AppKey
		}
		if in.AppSecret != "" {
			acc.AppSecret = in.AppSecret
		}
		if in.ConsumerKey != "" {
			acc.ConsumerKey = in.ConsumerKey
		}
		acc.IsDefault = acc.IsDefault || in.SetDefault

		if err := state.DB.UpsertAccount(acc); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		state.OVH.Invalidate(acc.ID)
		_ = state.ReloadAccounts()

		valid := verifyAccountCreds(state, acc.ID)
		c.JSON(http.StatusOK, gin.H{"account": acc, "valid": valid})
	}
}

// DeleteAccountByID DELETE /api/accounts/:id  级联删除
func DeleteAccountByID(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := state.DB.DeleteAccount(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		state.OVH.Invalidate(id)
		_ = state.ReloadAccounts()
		// 关联的内存数据也得清掉(queue / history / sniper_tasks)
		reloadAfterAccountDelete(state, id)
		state.Logger.Info("删除账户 + 级联清理: "+id, "accounts")
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

// SetDefaultAccountByID POST /api/accounts/:id/set-default
func SetDefaultAccountByID(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if err := state.DB.SetDefaultAccount(id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		_ = state.ReloadAccounts()
		c.JSON(http.StatusOK, gin.H{"status": "success"})
	}
}

// VerifyAccount POST /api/accounts/:id/verify
func VerifyAccount(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		c.JSON(http.StatusOK, gin.H{"valid": verifyAccountCreds(state, id)})
	}
}

// ── 内部工具 ───────────────────────────────────────────────────────────────

// verifyAccountCreds 用账户凭据调 OVH /me 验证有效
func verifyAccountCreds(state *app.State, accountID string) bool {
	cli, err := state.OVH.ClientFor(accountID)
	if err != nil {
		return false
	}
	var me map[string]interface{}
	if err := cli.Get("/me", &me); err != nil {
		state.Logger.Warn("verify account "+accountID+": "+err.Error(), "accounts")
		return false
	}
	return true
}

// reloadAfterAccountDelete 删账户后,把内存里关联的 queue/history/sniper_tasks
// 重新从 SQLite 加载(级联删除已经把这些行删掉了)
func reloadAfterAccountDelete(state *app.State, _ string) {
	if items, err := state.DB.ListQueue(); err == nil {
		state.QueueMu.Lock()
		state.Queue = items
		if state.Queue == nil {
			state.Queue = []types.QueueItem{}
		}
		state.QueueMu.Unlock()
	}
	if items, err := state.DB.ListHistory(); err == nil {
		state.HistoryMu.Lock()
		state.History = items
		if state.History == nil {
			state.History = []types.PurchaseHistoryEntry{}
		}
		state.HistoryMu.Unlock()
	}
	// 监控订阅的 auto_order_account_id 已经被 SQL UPDATE 清空了,
	// 但内存里还是旧值,得重载
	if subs, err := state.DB.ListMonitorSubscriptions(); err == nil {
		_ = subs // 实际由 monitor 包自己 LoadFromDB,这里跳过
	}
	if subs, err := state.DB.ListVPSSubscriptions(); err == nil {
		state.VPSSubsMu.Lock()
		state.VPSSubscriptions = subs
		if state.VPSSubscriptions == nil {
			state.VPSSubscriptions = []types.VPSSubscription{}
		}
		state.VPSSubsMu.Unlock()
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
