package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSupportTicketListQuery(t *testing.T) {
	q, err := supportTicketListQuery("open", " 16893164 ", "true", "2", "80")
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("status") != "open" || q.Get("ticketNumber") != "16893164" || q.Get("archived") != "true" {
		t.Fatalf("query = %s", q.Encode())
	}
	if q.Get("page") != "2" || q.Get("pageSize") != "50" {
		t.Fatalf("page query = %s", q.Encode())
	}
	if q.Get("subject") != "" {
		t.Fatal("数字搜索不该同时当作标题")
	}

	q, err = supportTicketListQuery("all", "refund two servers", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("status") != "" || q.Get("subject") != "refund two servers" || q.Get("page") != "1" || q.Get("pageSize") != "20" {
		t.Fatalf("default query = %s", q.Encode())
	}

	if _, err := supportTicketListQuery("nope", "", "", "", ""); err == nil {
		t.Fatal("非法状态应拒绝")
	}
	if _, err := supportTicketListQuery("", "", "maybe", "", ""); err == nil {
		t.Fatal("非法 archived 应拒绝")
	}
	if _, err := supportTicketListQuery("", "", "", "0", ""); err == nil {
		t.Fatal("0 页应拒绝")
	}
}

func TestBuildSupportCreatePayload(t *testing.T) {
	payload, err := buildSupportCreatePayload(supportCreateRequest{
		Subject:     " Please cancel two servers ",
		Body:        "Please refund both orders.",
		Category:    "billing",
		Subcategory: "bill",
		Product:     "dedicated",
		ServiceName: "ns3104399.ip-54-36-168.eu",
	})
	if err != nil {
		t.Fatal(err)
	}
	if payload["subject"] != "Please cancel two servers" || payload["serviceName"] != "ns3104399.ip-54-36-168.eu" {
		t.Fatalf("payload = %#v", payload)
	}
	if _, err := buildSupportCreatePayload(supportCreateRequest{Subject: "x", Body: "y", Category: "competitor"}); err == nil {
		t.Fatal("非法分类应拒绝")
	}
	if _, err := buildSupportCreatePayload(supportCreateRequest{Subject: " ", Body: "y"}); err == nil {
		t.Fatal("空标题应拒绝")
	}
	if _, err := buildSupportCreatePayload(supportCreateRequest{Subject: "x", Body: "y", ServiceName: "a b"}); err == nil {
		t.Fatal("带空格的服务名应拒绝")
	}
}

func TestParseSupportTicketID(t *testing.T) {
	id, err := parseSupportTicketID("16893164")
	if err != nil || id != "16893164" {
		t.Fatalf("id=%s err=%v", id, err)
	}
	for _, raw := range []string{"", "0", "01", "12a", "  "} {
		if _, err := parseSupportTicketID(raw); err == nil {
			t.Fatalf("%q 应拒绝", raw)
		}
	}
}

func TestCreateSupportTicketRejectsBadJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/support/tickets", CreateSupportTicket(newTestState(t)))
	req := httptest.NewRequest(http.MethodPost, "/api/support/tickets", strings.NewReader(`{"subject"`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestReplySupportTicketRejectsEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/support/tickets/:ticket_id/reply", ReplySupportTicket(newTestState(t)))
	req := httptest.NewRequest(http.MethodPost, "/api/support/tickets/16893164/reply", strings.NewReader(`{"body":"  "}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}
