package numconv

import "testing"

func TestToInt64(t *testing.T) {
	cases := []struct {
		in      interface{}
		want    int64
		wantOK  bool
	}{
		{1, 1, true},
		{int64(2), 2, true},
		{float64(3), 3, true},
		{"4", 4, true},
		{nil, 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		got, ok := ToInt64(c.in)
		if ok != c.wantOK || (c.wantOK && got != c.want) {
			t.Fatalf("ToInt64(%v)=(%d,%v) want (%d,%v)", c.in, got, ok, c.want, c.wantOK)
		}
	}
}

func TestToString(t *testing.T) {
	if ToString(int64(42)) != "42" {
		t.Fatal("ToString int64 failed")
	}
	if ToString("hi") != "hi" {
		t.Fatal("ToString string failed")
	}
}
