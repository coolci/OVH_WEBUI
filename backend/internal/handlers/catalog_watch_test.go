package handlers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCatalogWatchAPIRequiresExplicitBooleanAndPersistsOff(t *testing.T) {
	s := newTestState(t)
	r := gin.New()
	r.GET("/watch", GetCatalogWatch(s))
	r.PUT("/watch", SetCatalogWatch(s))
	get := httptest.NewRecorder()
	r.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/watch", nil))
	if get.Code != 200 || !strings.Contains(get.Body.String(), `"enabled":true`) || !strings.Contains(get.Body.String(), `"autoPay":false`) {
		t.Fatal(get.Body.String())
	}
	for _, body := range []string{`{}`, `{"enabled":null}`, `{"enabled":"false"}`} {
		req := httptest.NewRequest(http.MethodPut, "/watch", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 400 {
			t.Fatal("invalid toggle accepted")
		}
	}
	req := httptest.NewRequest(http.MethodPut, "/watch", strings.NewReader(`{"enabled":false,"autoPay":true}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"enabled":false`) || !strings.Contains(w.Body.String(), `"autoPay":false`) {
		t.Fatal(w.Body.String())
	}
}
