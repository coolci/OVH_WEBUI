package monitor

import "testing"

func TestSnapshotCopiesOptions(t *testing.T) {
	s := &Subscription{
		PlanCode: "24sk602",
		Options:  []string{"ram-64g-noecc-2133", "softraid-2x480ssd"},
	}
	snap := s.snapshot()
	if len(snap.Options) != 2 || snap.Options[0] != "ram-64g-noecc-2133" {
		t.Fatalf("snapshot 丢掉 Options: %+v", snap.Options)
	}
	snap.Options[0] = "mutated"
	if s.Options[0] != "ram-64g-noecc-2133" {
		t.Fatal("snapshot.Options 必须是拷贝，不能改到真身")
	}
}
