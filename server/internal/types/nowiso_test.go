package types

import (
	"strings"
	"testing"
	"time"
)

func TestNowISO_ContainsTimezone(t *testing.T) {
	ts := NowISO()
	// 应包含时区标识（+HH:MM 或 -HH:MM 或 Z）
	hasTz := strings.HasSuffix(ts, "Z") || strings.Contains(ts[len(ts)-6:], "+") || strings.Contains(ts[len(ts)-6:], "-")
	if !hasTz {
		t.Fatalf("NowISO() 输出 %q 缺少时区偏移", ts)
	}

	parsed, ok := ParseTS(ts)
	if !ok {
		t.Fatalf("ParseTS 无法解析 NowISO() 生成的时间戳 %q", ts)
	}
	if time.Since(parsed) > 5*time.Second || time.Since(parsed) < -5*time.Second {
		t.Fatalf("解析后的时间差过大: now=%v, parsed=%v", time.Now(), parsed)
	}
}

func TestParseTS_LegacyWithoutTimezone(t *testing.T) {
	legacy := "2026-11-12T21:40:00.123456"
	parsed, ok := ParseTS(legacy)
	if !ok {
		t.Fatalf("ParseTS 应当兼容旧版无时区时间戳 %q", legacy)
	}
	if parsed.Year() != 2026 || parsed.Month() != 11 || parsed.Day() != 12 || parsed.Hour() != 21 || parsed.Minute() != 40 {
		t.Fatalf("旧版时间戳解析结果错误: %v", parsed)
	}
}
