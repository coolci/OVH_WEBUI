package handlers

import (
	"fmt"
	"strings"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/monitor"
)

// A supplied list replaces the legacy single account, including an explicit empty
// list. Never substitute the default account for a deleted or misspelled ID.
func validatedMonitorAccounts(state *app.State, legacy string, ids []string) ([]string, error) {
	ids = monitor.NormalizeOrderAccounts(legacy, ids)
	for _, id := range ids {
		if _, ok := state.FindAccount(id); !ok {
			return nil, fmt.Errorf("自动下单账户 %s 不存在", id)
		}
	}
	return ids, nil
}

func preflightMonitorAccounts(mon *monitor.Monitor, planCode string, ids []string) (region, subsidiary, warning string) {
	if len(ids) == 0 {
		return mon.PreflightRegion(planCode, "")
	}
	warnings := []string{}
	for i, id := range ids {
		r, s, w := mon.PreflightRegion(planCode, id)
		if i == 0 {
			region, subsidiary = r, s
		}
		if w != "" {
			warnings = append(warnings, fmt.Sprintf("账户 %s: %s", id, w))
		}
	}
	return region, subsidiary, strings.Join(warnings, "；")
}
