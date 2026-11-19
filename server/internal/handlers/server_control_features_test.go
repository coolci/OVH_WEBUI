package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/types"
)

func TestBackupFTPSubEndpoints_Unactivated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)

	st.Accounts = []types.OVHAccount{
		{
			ID:          "test-acc",
			Name:        "Test Account",
			Endpoint:    "ovh-eu",
			Zone:        "FR",
			AppKey:      "test",
			AppSecret:   "test",
			ConsumerKey: "test",
			IsDefault:   true,
		},
	}

	client, err := st.OVH.ClientFor("test-acc")
	if err != nil {
		t.Fatal(err)
	}

	client.Client = &http.Client{
		Transport: queuePreorderTransport(func(req *http.Request) (*http.Response, error) {
			path := req.URL.Path
			if strings.HasSuffix(path, "/auth/time") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{},
					Body:       io.NopCloser(strings.NewReader(strconv.FormatInt(time.Now().Unix(), 10))),
					Request:    req,
				}, nil
			}

			// 服务器本身存在
			if strings.HasSuffix(path, "/dedicated/server/ns12345.ip") {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"name":"ns12345.ip","state":"ok"}`)),
					Request:    req,
				}, nil
			}

			// 备份 FTP 未激活，子接口返回 404 backupFTP does not exist
			if strings.Contains(path, "/features/backupFTP/access") || strings.Contains(path, "/features/backupFTP/authorizableBlocks") {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"message":"backupFTP does not exist"}`)),
					Request:    req,
				}, nil
			}

			return &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     http.Header{},
				Body:       io.NopCloser(strings.NewReader(`{"message":"not found"}`)),
				Request:    req,
			}, nil
		}),
	}

	router := gin.New()
	router.GET("/api/server-control/:service_name/backup-ftp/access", GetBackupFTPAccess(st))
	router.GET("/api/server-control/:service_name/backup-ftp/authorizable-blocks", GetBackupFTPAuthorizableBlocks(st))

	// 1. 验证 GetBackupFTPAccess
	{
		req := httptest.NewRequest(http.MethodGet, "/api/server-control/ns12345.ip/backup-ftp/access", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if notAct, _ := resp["notActivated"].(bool); !notAct {
			t.Fatalf("expected notActivated=true, got %+v", resp)
		}
		if errMsg, _ := resp["error"].(string); !strings.Contains(errMsg, "备份FTP未激活") {
			t.Fatalf("expected '备份FTP未激活', got %+v", resp)
		}
	}

	// 2. 验证 GetBackupFTPAuthorizableBlocks
	{
		req := httptest.NewRequest(http.MethodGet, "/api/server-control/ns12345.ip/backup-ftp/authorizable-blocks", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if notAct, _ := resp["notActivated"].(bool); !notAct {
			t.Fatalf("expected notActivated=true, got %+v", resp)
		}
		if errMsg, _ := resp["error"].(string); !strings.Contains(errMsg, "备份FTP未激活") {
			t.Fatalf("expected '备份FTP未激活', got %+v", resp)
		}
	}
}
