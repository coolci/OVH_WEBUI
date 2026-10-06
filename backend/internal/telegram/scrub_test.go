package telegram

import "testing"

func TestScrubRemovesToken(t *testing.T) {
	cases := []string{
		"Post \"https://api.telegram.org/bot123456789:AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw/sendMessage\": dial tcp: i/o timeout",
		"https://api.telegram.org/bot987654321:XYZ_abc-123/getWebhookInfo",
	}
	for _, in := range cases {
		got := scrub(in)
		if got == in {
			t.Errorf("没有脱敏: %s", got)
		}
		for _, leak := range []string{"AAHdqTcvCH1vGWJxfSeofSAs0K5PALDsaw", "XYZ_abc-123", "123456789:", "987654321:"} {
			if contains(got, leak) {
				t.Errorf("Token 片段仍然泄漏 %q 在 %q", leak, got)
			}
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
