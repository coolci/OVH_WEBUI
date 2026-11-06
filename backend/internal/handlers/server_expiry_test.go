package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOwnedServerListHidesOnlyConfirmedExpiry(t *testing.T) {
	st, router, _ := supportFixture(t)
	client, _ := st.OVH.ClientFor("eu")
	client.Client.Transport = queuePreorderTransport(func(r *http.Request) (*http.Response, error) {
		p := r.URL.Path
		body, code := `{}`, 200
		switch {
		case strings.HasSuffix(p, "/auth/time"):
			body = fmt.Sprint(time.Now().Unix())
		case strings.HasSuffix(p, "/dedicated/server"):
			body = `["expired-detail","expired-info","expired-status","active","credential-error","transient-error"]`
		case strings.HasSuffix(p, "/expired-detail"):
			code = 460
			body = `{"message":"This service is expired"}`
		case strings.HasSuffix(p, "/expired-info/serviceInfos"):
			code = 460
			body = `{"message":"This service is expired"}`
		case strings.HasSuffix(p, "/expired-status/serviceInfos"):
			body = `{"status":"expired"}`
		case strings.HasSuffix(p, "/credential-error"):
			code = 460
			body = `{"message":"Credential is expired"}`
		case strings.HasSuffix(p, "/transient-error/serviceInfos"):
			code = 503
			body = `{"message":"Service unavailable"}`
		case strings.HasSuffix(p, "/serviceInfos"):
			body = `{"status":"ok","expiration":"2000-01-01","renew":{"automatic":true}}`
		case strings.Contains(p, "/hardware"):
			code = 404
			body = `{"message":"Not available"}`
		default:
			body = `{"state":"ok"}`
		}
		return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	router.GET("/list", ListMyServers(st))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/list?account=eu", nil))
	var result struct {
		Total   int `json:"total"`
		Servers []struct {
			Name string `json:"serviceName"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil || response.Code != 200 {
		t.Fatalf("%d %s %v", response.Code, response.Body, err)
	}
	if result.Total != 3 || len(result.Servers) != 3 {
		t.Fatalf("unexpected visible servers: %s", response.Body)
	}
	for _, server := range result.Servers {
		if strings.HasPrefix(server.Name, "expired-") {
			t.Fatal("expired server leaked into list")
		}
	}
}
