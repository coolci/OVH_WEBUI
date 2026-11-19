package telegram

import "testing"

func TestParseGetUpdatesBody(t *testing.T) {
	body := []byte(`{"ok":true,"result":[{"update_id":101,"message":{"text":"/start"}},{"update_id":102,"callback_query":{"data":"x"}}]}`)
	ok, desc, updates, err := parseGetUpdatesBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || desc != "" {
		t.Fatalf("ok=%v desc=%q", ok, desc)
	}
	if len(updates) != 2 {
		t.Fatalf("len=%d", len(updates))
	}
	if ParseUpdateID(updates[0]["update_id"]) != 101 {
		t.Fatalf("id0=%v", updates[0]["update_id"])
	}
	if _, has := updates[1]["callback_query"]; !has {
		t.Fatal("expected callback_query")
	}
}

func TestParseGetUpdatesBodyError(t *testing.T) {
	body := []byte(`{"ok":false,"description":"Conflict: can't use getUpdates while webhook is active"}`)
	ok, desc, updates, err := parseGetUpdatesBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected not ok")
	}
	if desc == "" || len(updates) != 0 {
		t.Fatalf("desc=%q n=%d", desc, len(updates))
	}
}

func TestParseUpdateID(t *testing.T) {
	if ParseUpdateID(float64(12)) != 12 {
		t.Fatal("float64")
	}
	if ParseUpdateID(int64(13)) != 13 {
		t.Fatal("int64")
	}
}
