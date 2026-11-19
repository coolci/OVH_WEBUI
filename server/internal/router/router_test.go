package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func TestIsOriginAllowed(t *testing.T) {
	tests := []struct {
		origin string
		env    string
		want   bool
	}{
		{"", "", true},
		{"http://localhost:8080", "", true},
		{"http://127.0.0.1:3000", "", true},
		{"http://[::1]:19998", "", true},
		{"http://192.168.1.50:8080", "", true},
		{"http://10.0.0.2:8080", "", true},
		{"http://172.20.0.5:8080", "", true},
		{"http://evil.com", "", false},
		{"http://allowed.com", "http://allowed.com,http://other.com", true},
		{"invalid url %%#", "", false},
	}

	for _, tc := range tests {
		if tc.env != "" {
			_ = os.Setenv("CORS_ALLOWED_ORIGINS", tc.env)
		} else {
			_ = os.Unsetenv("CORS_ALLOWED_ORIGINS")
		}
		got := IsOriginAllowed(tc.origin)
		if got != tc.want {
			t.Errorf("IsOriginAllowed(%q) with env=%q: got %v, want %v", tc.origin, tc.env, got, tc.want)
		}
	}
}

func TestAllowedOrigins(t *testing.T) {
	origins := AllowedOrigins("19998")
	if len(origins) == 0 {
		t.Fatal("AllowedOrigins should not return empty slice")
	}
}

func TestMountEmbeddedUI(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	testFS := fstest.MapFS{
		"index.html":       {Data: []byte("<html><body>OVH WebUI</body></html>")},
		"assets/style.css": {Data: []byte("body { background: #000; }")},
	}

	MountEmbeddedUI(r, testFS, true)

	// Test index fallback
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "<html><body>OVH WebUI</body></html>" {
		t.Errorf("expected root to serve index.html, got code %d body %s", w.Code, w.Body.String())
	}

	// Test static file
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/assets/style.css", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "body { background: #000; }" {
		t.Errorf("expected static file serve, got code %d body %s", w.Code, w.Body.String())
	}

	// Test API 404 (must not serve index.html)
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", "/api/not-exist", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected API not found 404, got %d", w.Code)
	}
}
