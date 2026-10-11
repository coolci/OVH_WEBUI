package httperr_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ovh-webui/server/internal/platform/httperr"
)

func TestProblemDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, _ := http.NewRequest(http.MethodGet, "/api/test", nil)
	c.Request = req

	httperr.BadRequest(c, "invalid plan code parameter", "INVALID_PARAM")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var resp httperr.ProblemDetails
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if resp.Status != 400 || resp.Code != "INVALID_PARAM" || resp.Detail != "invalid plan code parameter" {
		t.Errorf("unexpected problem details body: %+v", resp)
	}
	if resp.Success {
		t.Errorf("expected success false, got true")
	}
	if resp.Instance != "/api/test" {
		t.Errorf("expected instance /api/test, got %s", resp.Instance)
	}
}
