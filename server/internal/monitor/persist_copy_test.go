package monitor

import "testing"

// toDBSub 的结果在锁外才序列化,必须和订阅真身不共享任何可变容器:
// 多账户检查合并结果、删除账户清引用都会在锁内原地改 LastStatus,
// 共用同一个 map 时并发就是 Go 运行时的 fatal error(recover 接不住)。
func TestToDBSubDoesNotShareMutableState(t *testing.T) {
	s := &Subscription{
		PlanCode:    "p",
		Datacenters: []string{"gra"},
		Options:     []string{"ram-a"},
		LastStatus:  map[string]string{"gra|c": "available"},
	}
	row := toDBSub(s)
	s.LastStatus["gra|c"] = "unavailable"
	s.LastStatus["rbx|c"] = "available"
	s.Datacenters[0] = "changed"
	s.Options[0] = "changed"
	if row.LastStatus["gra|c"] != "available" || len(row.LastStatus) != 1 {
		t.Fatalf("LastStatus 与真身共享: %v", row.LastStatus)
	}
	if row.Datacenters[0] != "gra" || row.Options[0] != "ram-a" {
		t.Fatalf("切片与真身共享: %v %v", row.Datacenters, row.Options)
	}
}

// 已经据此下单/通知的基线要立即生效,且不能原地改别人手里的旧 map。
func TestCommitStatusCopiesOnWrite(t *testing.T) {
	old := map[string]string{"gra|c": "unavailable"}
	s := &Subscription{PlanCode: "p", LastStatus: old}
	s.commitStatus(map[string]string{"gra|c": "available"})
	if old["gra|c"] != "unavailable" {
		t.Fatal("commitStatus 原地改了旧 map")
	}
	if s.statusSnapshot()["gra|c"] != "available" {
		t.Fatal("已提交的基线没有生效")
	}

	parent := &Subscription{PlanCode: "p"}
	check := &Subscription{PlanCode: "p", checkParent: parent}
	check.commitStatus(map[string]string{"gra|c": "available"})
	if got := check.takeCommitted(); got["gra|c"] != "available" {
		t.Fatalf("账户副本应记下已提交的基线: %v", got)
	}
	if check.takeCommitted() != nil {
		t.Fatal("takeCommitted 应取走记录")
	}
}

// revision 变化时整轮作废,但已经据此下单的账户基线必须合并回父订阅;
// 已从订阅里移除的账户不能被复活。
func TestMergeCommittedChecksOnlyLiveAccounts(t *testing.T) {
	m := &Monitor{}
	sub := &Subscription{PlanCode: "p", AutoOrderAccountIDs: []string{"a"}, AutoOrderAccountID: "a", LastStatus: map[string]string{}}
	ca := &Subscription{PlanCode: "p", checkParent: sub}
	cb := &Subscription{PlanCode: "p", checkParent: sub}
	ca.commitStatus(map[string]string{"gra|c": "available"})
	cb.commitStatus(map[string]string{"gra|c": "available"})
	m.mergeCommittedChecks(sub, []string{"a", "b"}, []*Subscription{ca, cb})
	got := sub.statusSnapshot()
	if got[accountStatusPrefix("a")+"gra|c"] != "available" {
		t.Fatalf("账户 a 的已提交基线没合并: %v", got)
	}
	if _, ok := got[accountStatusPrefix("b")+"gra|c"]; ok {
		t.Fatalf("已移除的账户 b 被复活: %v", got)
	}
	if !m.dirty.Load() {
		t.Fatal("合并后应标脏落库")
	}
}
