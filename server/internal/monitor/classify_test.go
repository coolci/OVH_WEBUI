package monitor

import (
	"testing"
)

func TestClassifyPriceCheckError(t *testing.T) {
	tests := []struct {
		errStr       string
		expectedKind PriceCheckErrorKind
	}{
		{
			errStr:       `OVHcloud API error (status code 400): Client::BadRequest: "Plan code not found 27sysle01-v1"`,
			expectedKind: ErrorKindPreSale,
		},
		{
			errStr:       "在 Eco 目录里未找到 planCode: 27sysstorle01-v1",
			expectedKind: ErrorKindPreSale,
		},
		{
			errStr:       "机型 24sys032-v1 不在该账户所属子公司 FR（EU 区）的目录里：OVH 的 EU / US / CA 三个站点目录互不相通",
			expectedKind: ErrorKindRegionBlocked,
		},
		{
			errStr:       "HTTP 429: Too Many Requests",
			expectedKind: ErrorKindTransient,
		},
		{
			errStr:       "context deadline exceeded",
			expectedKind: ErrorKindTransient,
		},
		{
			errStr:       "random unexpected error",
			expectedKind: ErrorKindUnknown,
		},
	}

	for _, tt := range tests {
		kind, _ := ClassifyPriceCheckError(tt.errStr)
		if kind != tt.expectedKind {
			t.Errorf("ClassifyPriceCheckError(%q) = %v; want %v", tt.errStr, kind, tt.expectedKind)
		}
	}
}

