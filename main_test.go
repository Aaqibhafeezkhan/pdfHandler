package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func TestHealthz(t *testing.T) {
	r, err := newRouter()
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz = %d, want 200", rec.Code)
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding %q: %v", rec.Body.String(), err)
	}
	if got["status"] != "ok" {
		t.Errorf(`status = %q, want "ok"`, got["status"])
	}
}

// The UI inlines its own CSS and JS, so the CSP has to allow 'unsafe-inline'
// for those two directives — but nothing should be loadable from another
// origin, and the page must not be framable.
func TestSecurityHeaders(t *testing.T) {
	r, err := newRouter()
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	want := map[string]string{
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
		"Referrer-Policy":            "no-referrer",
		"Cross-Origin-Opener-Policy": "same-origin",
	}
	for header, value := range want {
		if got := rec.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy is missing")
	}
	for _, directive := range []string{
		"default-src 'self'",
		"object-src 'none'",
		"base-uri 'none'",
		"frame-ancestors 'none'",
		"form-action 'self'",
	} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP is missing %q: %s", directive, csp)
		}
	}
}

// Uploaded documents are private; a POST response must not be cached by the
// browser or an intermediate proxy.
func TestPostResponsesAreNoStore(t *testing.T) {
	r, err := newRouter()
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/merge", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control on a POST = %q, want %q", got, "no-store")
	}
}

// bodyLimit is the only thing standing between an upload and the container's
// disk: MaxMultipartMemory just decides how much is held in RAM first.
func TestBodyLimitTruncatesOversizeRequests(t *testing.T) {
	const limit = 64

	r := gin.New()
	r.Use(bodyLimit(limit))

	var readErr error
	r.POST("/x", func(c *gin.Context) {
		_, readErr = io.ReadAll(c.Request.Body)
		c.Status(http.StatusOK)
	})

	t.Run("over the limit", func(t *testing.T) {
		readErr = nil
		body := bytes.Repeat([]byte("a"), limit*4)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", bytes.NewReader(body)))

		if readErr == nil {
			t.Fatalf("reading a %d-byte body under a %d-byte limit succeeded", len(body), limit)
		}
		var maxBytes *http.MaxBytesError
		if !errors.As(readErr, &maxBytes) {
			t.Fatalf("got %v, want a *http.MaxBytesError so handlers can answer 413", readErr)
		}
	})

	t.Run("under the limit", func(t *testing.T) {
		readErr = nil
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("small")))

		if readErr != nil {
			t.Fatalf("a body under the limit failed to read: %v", readErr)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})
}

// Static assets are served from disk relative to the working directory, so this
// also catches a container that was built without ./static.
func TestServesUI(t *testing.T) {
	r, err := newRouter()
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<html") {
		t.Error("GET / did not return the UI")
	}
}
