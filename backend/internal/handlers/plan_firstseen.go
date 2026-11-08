package handlers

import (
	"sync"
	"time"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// 记录每个型号第一次出现在销售目录里的时间,服务器列表据此把新上架的排在最前面。
// OVH 目录本身没有上架时间(commercial.features 的 release-year 只有少数型号有,且只到年份)。
//
// 每个目录视角(默认账户 / 各子公司账户)第一次被记录时,里面已有的型号全部记 0 ——
// 它们在本程序开始记录之前就在卖了,不能因为"第一次看到"就全被当成新品顶上来。
// 之后这个视角里新冒出来的型号才记当前时间。

const kvPlanFirstSeen = "catalog_plan_first_seen"

type planFirstSeenData struct {
	Buckets map[string]bool  `json:"buckets"` // 已建立基线的目录视角
	Plans   map[string]int64 `json:"plans"`   // planCode → unix 秒;0 = 基线时已在售
}

var planFirstSeenMu sync.Mutex

func stampPlanFirstSeen(state *app.State, bucket string, plans []types.ServerPlan) map[string]int64 {
	if state == nil || state.DB == nil || len(plans) == 0 {
		return nil
	}
	planFirstSeenMu.Lock()
	defer planFirstSeenMu.Unlock()

	var data planFirstSeenData
	if _, err := state.DB.GetKV(kvPlanFirstSeen, &data); err != nil {
		return nil
	}
	if data.Buckets == nil {
		data.Buckets = map[string]bool{}
	}
	if data.Plans == nil {
		data.Plans = map[string]int64{}
	}
	baseline := !data.Buckets[bucket]
	now := time.Now().Unix()
	changed := baseline
	data.Buckets[bucket] = true
	for _, p := range plans {
		if p.PlanCode == "" {
			continue
		}
		if _, ok := data.Plans[p.PlanCode]; ok {
			continue
		}
		if baseline {
			data.Plans[p.PlanCode] = 0
		} else {
			data.Plans[p.PlanCode] = now
		}
		changed = true
	}
	if changed {
		if err := state.DB.SetKV(kvPlanFirstSeen, data); err != nil {
			state.Logger.Warn("记录型号首次上架时间失败: "+err.Error(), "servers")
		}
	}
	out := make(map[string]int64, len(plans))
	for _, p := range plans {
		out[p.PlanCode] = data.Plans[p.PlanCode]
	}
	return out
}
