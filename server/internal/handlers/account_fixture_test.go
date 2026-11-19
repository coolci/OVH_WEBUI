package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ovh-webui/server/internal/app"
	"github.com/ovh-webui/server/internal/types"
)

// supportCall 记录离线 transport 截获的一次 SDK 请求。
type supportCall struct {
	Method, Path, Host, Query string
	Body                      map[string]interface{}
}

// supportFixture 虚构 EU/US 两个账户并装上离线 transport,绝不访问真实 OVH;
// 需要特定响应的测试再自行替换 client.Client.Transport。
func supportFixture(t *testing.T) (*app.State, *gin.Engine, struct{}) {
	t.Helper()
	st, _ := newWatchTestMonitor(t)
	st.Accounts = []types.OVHAccount{
		{ID: "eu", Name: "EU fixture", Endpoint: "ovh-eu", Zone: "IE", AppKey: "test", AppSecret: "test", ConsumerKey: "test", IsDefault: true},
		{ID: "us", Name: "US fixture", Endpoint: "ovh-us", Zone: "US", AppKey: "test", AppSecret: "test", ConsumerKey: "test"},
	}
	for _, acc := range st.Accounts {
		client, err := st.OVH.ClientFor(acc.ID)
		if err != nil {
			t.Fatal(err)
		}
		client.Client = &http.Client{Transport: queuePreorderTransport(func(req *http.Request) (*http.Response, error) {
			body := `[]`
			if strings.HasSuffix(req.URL.Path, "/auth/time") {
				body = fmt.Sprint(time.Now().Unix())
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		})}
	}
	return st, gin.New(), struct{}{}
}
