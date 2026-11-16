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
	Buckets     map[string]bool             `json:"buckets"`               // 已建立基线的目录视角
	BucketPlans map[string]map[string]int64 `json:"bucketPlans,omitempty"` // 按子公司/视角独立隔离的首次出现时间戳字典
	Plans       map[string]int64            `json:"plans,omitempty"`       // 兼容旧落盘结构 & 全局已确认新品首次上架时间(>0)
}

var planFirstSeenMu sync.Mutex

func stampPlanFirstSeen(state *app.State, bucket string, plans []types.ServerPlan) map[string]int64 {
	if state == nil || !state.HasDB() || len(plans) == 0 {
		return nil
	}
	planFirstSeenMu.Lock()
	defer planFirstSeenMu.Unlock()

	var data planFirstSeenData
	if _, err := state.GetKV(kvPlanFirstSeen, &data); err != nil {
		return nil
	}
	if data.Buckets == nil {
		data.Buckets = map[string]bool{}
	}
	if data.BucketPlans == nil {
		data.BucketPlans = map[string]map[string]int64{}
	}
	if data.Plans == nil {
		data.Plans = map[string]int64{}
	}

	// 平滑迁移旧版扁平 JSON：仅对已建立过基线且尚未拆出独立分桶的旧 bucket 继承历史快照
	if data.Buckets[bucket] && data.BucketPlans[bucket] == nil {
		migrated := make(map[string]int64, len(data.Plans))
		for k, v := range data.Plans {
			migrated[k] = v
		}
		data.BucketPlans[bucket] = migrated
	}
	bucketMap := data.BucketPlans[bucket]
	if bucketMap == nil {
		bucketMap = map[string]int64{}
		data.BucketPlans[bucket] = bucketMap
	}

	baseline := !data.Buckets[bucket]
	now := time.Now().Unix()
	changed := baseline
	data.Buckets[bucket] = true
	for _, p := range plans {
		if p.PlanCode == "" {
			continue
		}
		if _, ok := bucketMap[p.PlanCode]; ok {
			continue
		}
		if baseline {
			// 新视角首次建立基线：若其他已建基线的视角已经捕获到它是上架新品(>0)，沿用该首次发现时间；
			// 否则仅在当前视角记 0，绝不向全局 Plans 写入 0 以免污染其他已有基线视角的新品判定！
			if confirmedTs, ok := data.Plans[p.PlanCode]; ok && confirmedTs > 0 {
				bucketMap[p.PlanCode] = confirmedTs
			} else {
				bucketMap[p.PlanCode] = 0
			}
		} else {
			ts := now
			if confirmedTs, ok := data.Plans[p.PlanCode]; ok && confirmedTs > 0 && confirmedTs < ts {
				ts = confirmedTs
			}
			bucketMap[p.PlanCode] = ts
			if existing, ok := data.Plans[p.PlanCode]; !ok || existing <= 0 {
				data.Plans[p.PlanCode] = ts
			}
		}
		changed = true
	}
	if changed {
		if err := state.SetKV(kvPlanFirstSeen, data); err != nil {
			state.Logger.Warn("记录型号首次上架时间失败: "+err.Error(), "servers")
		}
	}
	out := make(map[string]int64, len(plans))
	for _, p := range plans {
		out[p.PlanCode] = bucketMap[p.PlanCode]
	}
	return out
}
