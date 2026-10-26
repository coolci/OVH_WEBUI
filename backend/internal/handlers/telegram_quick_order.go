package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/monitor"
	"github.com/ovh-webui/server/internal/telegram"
	"github.com/ovh-webui/server/internal/types"
)

// TelegramQuickOrder POST /api/telegram/quick-order
// 网页「云下单」执行与 Bot 相同的命令语义（支持直接指令输入与结构化表单下单）。
// body: { rawCommand?, mode?, planCode?, datacenter?, quantity?, options?, accountRef? }
func TelegramQuickOrder(state *app.State, mon *monitor.Monitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			RawCommand string   `json:"rawCommand"`
			Mode       string   `json:"mode"`
			PlanCode   string   `json:"planCode"`
			Datacenter string   `json:"datacenter"`
			Quantity   int      `json:"quantity"`
			Options    []string `json:"options"`
			AccountRef string   `json:"accountRef"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "无效的请求体"})
			return
		}

		raw := strings.TrimSpace(body.RawCommand)
		if raw != "" {
			// 1) 斜杠命令直接派发 (/buy, /stock, /tasks, /accounts, /subs, /cancel, /interval...)
			if strings.HasPrefix(raw, "/") {
				cmd := telegram.ParseBotCommand(raw)
				if cmd == nil {
					c.JSON(http.StatusOK, gin.H{
						"success": false,
						"message": "❌ 命令格式无法解析: " + raw,
						"error":   "命令格式无法解析",
						"command": raw,
					})
					return
				}
				reply := dispatchTelegramCommand(state, mon, cmd)
				success := !strings.HasPrefix(strings.TrimSpace(reply), "❌")
				errField := ""
				if !success {
					errField = reply
				}
				c.JSON(http.StatusOK, gin.H{
					"success": success,
					"message": reply,
					"error":   errField,
					"mode":    cmd.Name,
					"command": cmd.Raw,
				})
				return
			}

			// 2) 中文别名或快捷方式
			lower := strings.ToLower(raw)
			var cmd *telegram.BotCommand
			switch {
			case lower == "任务" || lower == "队列" || lower == "任务列表":
				cmd = &telegram.BotCommand{Name: "tasks", Raw: raw}
			case lower == "账户" || lower == "账号" || lower == "切换账户":
				cmd = &telegram.BotCommand{Name: "accounts", Raw: raw}
			case lower == "监控" || lower == "监控列表" || lower == "订阅":
				cmd = &telegram.BotCommand{Name: "subs", Raw: raw}
			case lower == "库存" || lower == "查库存":
				cmd = &telegram.BotCommand{Name: "stock", Raw: raw}
			case lower == "帮助" || lower == "help" || lower == "?":
				cmd = &telegram.BotCommand{Name: "help", Raw: raw}
			case strings.HasPrefix(lower, "查库存 ") || strings.HasPrefix(lower, "库存 "):
				fields := strings.Fields(raw)
				if len(fields) > 1 {
					cmd = &telegram.BotCommand{Name: "stock", Args: fields[1:], Raw: raw}
				}
			case strings.HasPrefix(lower, "监控 "):
				fields := strings.Fields(raw)
				if len(fields) > 1 {
					cmd = &telegram.BotCommand{Name: "monitor", Args: fields[1:], Raw: raw}
				}
			case strings.HasPrefix(lower, "价格 ") || strings.HasPrefix(lower, "查价格 "):
				fields := strings.Fields(raw)
				if len(fields) > 1 {
					cmd = &telegram.BotCommand{Name: "price", Args: fields[1:], Raw: raw}
				}
			case strings.HasPrefix(lower, "买 ") || strings.HasPrefix(lower, "抢 ") || strings.HasPrefix(lower, "下单 "):
				fields := strings.Fields(raw)
				if len(fields) > 1 {
					cmd = &telegram.BotCommand{Name: "buy", Args: fields[1:], Raw: raw}
				}
			case strings.HasPrefix(lower, "取消 ") || lower == "取消":
				fields := strings.Fields(raw)
				if len(fields) > 1 {
					cmd = &telegram.BotCommand{Name: "cancel", Args: fields[1:], Raw: raw}
				} else {
					cmd = &telegram.BotCommand{Name: "cancel", Raw: raw}
				}
			}
			if cmd != nil {
				reply := dispatchTelegramCommand(state, mon, cmd)
				success := !strings.HasPrefix(strings.TrimSpace(reply), "❌")
				errField := ""
				if !success {
					errField = reply
				}
				c.JSON(http.StatusOK, gin.H{
					"success": success,
					"message": reply,
					"error":   errField,
					"mode":    cmd.Name,
					"command": raw,
				})
				return
			}

			// 3) free-form 下单: planCode [dc] [qty] [options] [@account]
			orderInfo := telegram.ParseOrderMessage(raw)
			if orderInfo != nil && orderInfo.PlanCode != "" && !strings.HasPrefix(orderInfo.PlanCode, "/") {
				reply := cmdBuyOrQueueInfo(state, mon, orderInfo, "buy")
				success := !strings.HasPrefix(strings.TrimSpace(reply), "❌")
				errField := ""
				if !success {
					errField = reply
				}
				c.JSON(http.StatusOK, gin.H{
					"success": success,
					"message": reply,
					"error":   errField,
					"mode":    "buy",
					"command": raw,
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": "❌ 未知输入格式\n\n" + telegram.HelpMessage(),
				"error":   "未知输入格式",
				"command": raw,
			})
			return
		}

		mode := strings.ToLower(strings.TrimSpace(body.Mode))
		planCode := strings.TrimSpace(body.PlanCode)
		dc := strings.ToLower(strings.TrimSpace(body.Datacenter))

		if mode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 mode"})
			return
		}
		if mode == "help" || mode == "start" {
			c.JSON(http.StatusOK, gin.H{"success": true, "message": telegram.HelpMessage()})
			return
		}

		// 结构化模式：tasks / accounts / subs 不需要 planCode
		if mode == "tasks" || mode == "accounts" || mode == "subs" {
			cmd := &telegram.BotCommand{
				Name: mode,
				Raw:  "/" + mode,
			}
			reply := dispatchTelegramCommand(state, mon, cmd)
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": reply,
				"mode":    mode,
				"command": "/" + mode,
			})
			return
		}

		if planCode == "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "缺少 planCode"})
			return
		}

		args, errMsg := buildTelegramCommandArgs(mode, planCode, dc, body.Quantity, body.Options, body.AccountRef)
		if errMsg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": errMsg})
			return
		}

		cmd := &telegram.BotCommand{
			Name: mode,
			Args: args,
			Raw:  "/" + mode + " " + strings.Join(args, " "),
		}
		reply := dispatchTelegramCommand(state, mon, cmd)
		success := !strings.HasPrefix(strings.TrimSpace(reply), "❌")
		errField := ""
		if !success {
			errField = reply
		}
		c.JSON(http.StatusOK, gin.H{
			"success": success,
			"message": reply,
			"error":   errField,
			"mode":    mode,
			"command": cmd.Raw,
		})
	}
}

func buildTelegramCommandArgs(mode, planCode, dc string, quantity int, options []string, accountRef string) ([]string, string) {
	accountRef = strings.TrimSpace(accountRef)
	var accArg string
	if accountRef != "" {
		accArg = "@" + strings.TrimPrefix(accountRef, "@")
	}

	switch mode {
	case "stock":
		args := []string{planCode}
		if accArg != "" {
			args = append(args, accArg)
		}
		return args, ""
	case "queue", "buy":
		args := []string{planCode}
		if dc != "" {
			args = append(args, dc)
		}
		if quantity > 1 {
			args = append(args, strconv.Itoa(quantity))
		}
		if accArg != "" {
			args = append(args, accArg)
		}
		var cleanOpts []string
		for _, o := range options {
			cleanOpts = append(cleanOpts, types.SplitList(o)...)
		}
		if len(cleanOpts) > 0 {
			args = append(args, strings.Join(cleanOpts, ","))
		}
		return args, ""
	case "monitor":
		args := []string{planCode}
		if dc != "" {
			for _, part := range types.SplitList(dc) {
				part = strings.TrimSpace(part)
				if part != "" {
					args = append(args, part)
				}
			}
		}
		return args, ""
	case "watch":
		args := []string{planCode}
		if dc != "" {
			for _, part := range types.SplitList(dc) {
				part = strings.TrimSpace(part)
				if part != "" {
					args = append(args, part)
				}
			}
		}
		if accArg != "" {
			args = append(args, accArg)
		}
		if quantity > 0 {
			args = append(args, fmt.Sprintf("x%d", quantity))
		}
		return args, ""
	case "price":
		if dc == "" {
			return nil, "price 模式需要 datacenter"
		}
		args := []string{planCode, dc}
		if accArg != "" {
			args = append(args, accArg)
		}
		return args, ""
	case "tasks", "accounts", "subs":
		return nil, ""
	default:
		return nil, "未知 mode: " + mode + "（支持 stock/queue/buy/monitor/price/watch/tasks/accounts/subs）"
	}
}

// RegisterTelegramCommands POST /api/telegram/register-commands
// 单独注册 Bot 命令菜单。
func RegisterTelegramCommands(state *app.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if errMsg := telegram.SetMyCommands(state); errMsg != "" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": errMsg})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "Bot 命令菜单已注册"})
	}
}

