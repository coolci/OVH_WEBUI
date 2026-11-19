package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/types"
)

func TestBatchAddAll_RejectsAutoOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, mon := newWatchTestMonitor(t)
	st.ServerPlans = []types.ServerPlan{{PlanCode: "24sk202", Name: "SYS-1"}}

	router := gin.New()
	router.POST("/api/monitor/subscriptions/batch-add-all", BatchAddAll(st, mon))

	// 请求开启 autoOrder
	reqBody := `{"autoOrder": true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/monitor/subscriptions/batch-add-all", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for batch-add-all with autoOrder=true, got %d (body: %s)", w.Code, w.Body.String())
	}

	// 确认没有任何订阅被添加
	if len(mon.Snapshot()) != 0 {
		t.Fatalf("expected 0 subscriptions, got %d", len(mon.Snapshot()))
	}
}

func TestBatchAddAllVPS_RejectsAutoOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, _ := newWatchTestMonitor(t)

	router := gin.New()
	router.POST("/api/vps-monitor/subscriptions/batch-add-all", BatchAddAllVPS(st))

	reqBody := `{"autoOrder": true}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/vps-monitor/subscriptions/batch-add-all", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for vps batch-add-all with autoOrder=true, got %d (body: %s)", w.Code, w.Body.String())
	}
}
