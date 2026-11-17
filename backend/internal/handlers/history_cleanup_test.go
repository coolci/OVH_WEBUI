package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/types"
)

func TestRemovePurchaseHistoryOnlyDeletesExactLocalID(t *testing.T) {
	s := newTestState(t)
	s.History = []types.PurchaseHistoryEntry{{ID: "history-1", AccountID: "a", OrderID: "123"}, {ID: "history-12", AccountID: "b", OrderID: "123"}}
	if err := s.SaveHistory(); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.DELETE("/purchase-history/:id", RemovePurchaseHistory(s))
	for _, id := range []string{"history-1", "history-1", "not-found"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/purchase-history/"+id, nil))
		if w.Code != http.StatusOK {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if rows := s.HistorySnapshot(); len(rows) != 1 || rows[0].ID != "history-12" {
		t.Fatal(rows)
	}
}

func TestRemovePurchaseHistoryReportsPersistenceFailure(t *testing.T) {
	s := newTestState(t)
	s.History = []types.PurchaseHistoryEntry{{ID: "keep"}}
	s.DB.Close()
	r := gin.New()
	r.DELETE("/purchase-history/:id", RemovePurchaseHistory(s))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/purchase-history/keep", nil))
	if w.Code != http.StatusInternalServerError || len(s.HistorySnapshot()) != 1 {
		t.Fatal(w.Code, w.Body.String())
	}
}
