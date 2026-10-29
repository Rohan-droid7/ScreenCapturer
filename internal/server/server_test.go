package server

import (
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ScreenCapturer/internal/auth"
	"ScreenCapturer/internal/clipboard"
	"ScreenCapturer/internal/capture"
	"ScreenCapturer/internal/config"
)

type mockCapturer struct {
	img      *image.RGBA
	err      error
	callCnt  int
	failOnce bool
}

func (m *mockCapturer) Name() string { return "mock" }
func (m *mockCapturer) Close() error { return nil }

func (m *mockCapturer) Capture() (*image.RGBA, error) {
	m.callCnt++
	if m.failOnce && m.callCnt == 1 {
		return nil, errors.New("temporary device glitch")
	}
	if m.err != nil {
		return nil, m.err
	}
	return m.img, nil
}

func setupTestServer(mockCap capture.Capturer, token string) (*Server, *auth.Manager) {
	cfg := config.DefaultConfig()
	cfg.Token = token
	authMgr := auth.NewManager(func() string { return cfg.Token }, func() string { return cfg.PIN }, cfg.AllowedHosts, cfg.Port)
	srv := NewServer(cfg, mockCap, authMgr)
	return srv, authMgr
}

func TestServerStatusEndpoint(t *testing.T) {
	mockCap := &mockCapturer{img: createTestImage(10, 10)}
	srv, _ := setupTestServer(mockCap, "test-token")

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()

	srv.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if res["status"] != "running" {
		t.Errorf("expected status 'running', got %v", res["status"])
	}
	if res["engine"] != "mock" {
		t.Errorf("expected engine 'mock', got %v", res["engine"])
	}
}

func TestServerCaptureUnauthorized(t *testing.T) {
	mockCap := &mockCapturer{img: createTestImage(10, 10)}
	srv, _ := setupTestServer(mockCap, "secret-token")

	tests := []struct {
		name       string
		authHeader string
		host       string
		method     string
		wantCode   int
	}{
		{
			name:       "Missing auth header",
			authHeader: "",
			host:       "localhost:8765",
			method:     "POST",
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "Invalid bearer token",
			authHeader: "Bearer wrong-token",
			host:       "localhost:8765",
			method:     "POST",
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "Invalid method GET on /api/capture",
			authHeader: "Bearer secret-token",
			host:       "localhost:8765",
			method:     "GET",
			wantCode:   http.StatusMethodNotAllowed,
		},
		{
			name:       "Invalid host header",
			authHeader: "Bearer secret-token",
			host:       "attacker.com",
			method:     "POST",
			wantCode:   http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/capture", nil)
			req.Host = tt.host
			req.RemoteAddr = "127.0.0.1:50000"
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			srv.handleCapture(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("got status %d, want %d", w.Code, tt.wantCode)
			}
		})
	}
}

func TestServerCaptureSuccessAndRetry(t *testing.T) {
	// Test that if capture fails once, retry succeeds!
	mockCap := &mockCapturer{
		img:      createTestImage(50, 50),
		failOnce: true,
	}
	srv, _ := setupTestServer(mockCap, "valid-token")

	req := httptest.NewRequest("POST", "/api/capture?format=png", nil)
	req.Host = "127.0.0.1:8765"
	req.RemoteAddr = "127.0.0.1:50000"
	req.Header.Set("Authorization", "Bearer valid-token")

	w := httptest.NewRecorder()
	srv.handleCapture(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 after retry, got %d. Body: %s", w.Code, w.Body.String())
	}

	if mockCap.callCnt != 2 {
		t.Errorf("expected 2 calls to Capturer (1 fail + 1 retry), got %d", mockCap.callCnt)
	}

	if w.Header().Get("Content-Type") != "image/png" {
		t.Errorf("expected image/png content type, got %s", w.Header().Get("Content-Type"))
	}
}

func TestServerCaptureLockedScreenError(t *testing.T) {
	mockCap := &mockCapturer{
		err: capture.ErrScreenLocked,
	}
	srv, _ := setupTestServer(mockCap, "valid-token")

	req := httptest.NewRequest("POST", "/api/capture", nil)
	req.Host = "127.0.0.1:8765"
	req.RemoteAddr = "127.0.0.1:50000"
	req.Header.Set("Authorization", "Bearer valid-token")

	w := httptest.NewRecorder()
	srv.handleCapture(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected HTTP 503 Service Unavailable, got %d", w.Code)
	}

	var res errorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode JSON error: %v", err)
	}

	if res.Code != "SCREEN_LOCKED" {
		t.Errorf("expected code SCREEN_LOCKED, got %s", res.Code)
	}
}

func init() {
	_ = color.RGBA{}
}

func TestServerLockUnauthorized(t *testing.T) {
	mockCap := &mockCapturer{img: createTestImage(10, 10)}
	srv, _ := setupTestServer(mockCap, "secret-token")

	tests := []struct {
		name       string
		authHeader string
		host       string
		method     string
		wantCode   int
	}{
		{
			name:       "Missing auth header",
			authHeader: "",
			host:       "localhost:8765",
			method:     "POST",
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "Invalid bearer token",
			authHeader: "Bearer wrong-token",
			host:       "localhost:8765",
			method:     "POST",
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "Invalid method GET on /api/lock",
			authHeader: "Bearer secret-token",
			host:       "localhost:8765",
			method:     "GET",
			wantCode:   http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/lock", nil)
			req.Host = tt.host
			req.RemoteAddr = "127.0.0.1:50000"
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()
			srv.handleLockPC(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("expected status %d, got %d", tt.wantCode, w.Code)
			}
		})
	}
}

func TestServerLockSuccess(t *testing.T) {
	mockCap := &mockCapturer{img: createTestImage(10, 10)}
	srv, _ := setupTestServer(mockCap, "secret-token")

	called := 0
	srv.SetLockFunc(func() error {
		called++
		return nil
	})

	req := httptest.NewRequest("POST", "/api/lock", nil)
	req.Host = "127.0.0.1:8765"
	req.RemoteAddr = "127.0.0.1:50000"
	req.Header.Set("Authorization", "Bearer secret-token")

	w := httptest.NewRecorder()
	srv.handleLockPC(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	if called != 1 {
		t.Errorf("expected lockFunc to be called once, got %d", called)
	}

	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["status"] != "locked" {
		t.Errorf("expected status 'locked', got %s", res["status"])
	}
}

func TestServerLockFailure(t *testing.T) {
	mockCap := &mockCapturer{img: createTestImage(10, 10)}
	srv, _ := setupTestServer(mockCap, "secret-token")

	srv.SetLockFunc(func() error {
		return errors.New("mock OS lock error")
	})

	req := httptest.NewRequest("POST", "/api/lock", nil)
	req.Host = "127.0.0.1:8765"
	req.RemoteAddr = "127.0.0.1:50000"
	req.Header.Set("Authorization", "Bearer secret-token")

	w := httptest.NewRecorder()
	srv.handleLockPC(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got %d", w.Code)
	}

	var res errorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if res.Code != "LOCK_FAILED" {
		t.Errorf("expected code LOCK_FAILED, got %s", res.Code)
	}
}

func TestServerClipboardUnauthorized(t *testing.T) {
	mockCap := &mockCapturer{img: createTestImage(10, 10)}
	srv, _ := setupTestServer(mockCap, "secret-token")

	tests := []struct {
		name       string
		authHeader string
		method     string
		wantCode   int
	}{
		{
			name:       "Missing auth header on GET",
			authHeader: "",
			method:     "GET",
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "Invalid bearer token on GET",
			authHeader: "Bearer bad-token",
			method:     "GET",
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "Missing auth header on POST",
			authHeader: "",
			method:     "POST",
			wantCode:   http.StatusUnauthorized,
		},
		{
			name:       "Invalid method PUT",
			authHeader: "Bearer secret-token",
			method:     "PUT",
			wantCode:   http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/clipboard", nil)
			req.Host = "localhost:8765"
			req.RemoteAddr = "127.0.0.1:50000"
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			w := httptest.NewRecorder()
			srv.handleClipboard(w, req)

			if w.Code != tt.wantCode {
				t.Errorf("expected status %d, got %d", tt.wantCode, w.Code)
			}
		})
	}
}

func TestServerClipboardGet(t *testing.T) {
	mockCap := &mockCapturer{img: createTestImage(10, 10)}
	srv, _ := setupTestServer(mockCap, "secret-token")

	sampleText := "PC clipboard snippet 🚀"
	srv.SetClipboardFuncs(func() (string, error) {
		return sampleText, nil
	}, nil)

	req := httptest.NewRequest("GET", "/api/clipboard", nil)
	req.Host = "127.0.0.1:8765"
	req.RemoteAddr = "127.0.0.1:50000"
	req.Header.Set("Authorization", "Bearer secret-token")

	w := httptest.NewRecorder()
	srv.handleClipboard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var res clipboard.Content
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode clipboard JSON: %v", err)
	}

	if res.Text != sampleText {
		t.Errorf("expected text %q, got %q", sampleText, res.Text)
	}
	if !res.HasText {
		t.Errorf("expected has_text to be true")
	}
	if res.Length != len([]rune(sampleText)) {
		t.Errorf("expected length %d, got %d", len([]rune(sampleText)), res.Length)
	}
}

func TestServerClipboardPostJSON(t *testing.T) {
	mockCap := &mockCapturer{img: createTestImage(10, 10)}
	srv, _ := setupTestServer(mockCap, "secret-token")

	var written string
	srv.SetClipboardFuncs(nil, func(s string) error {
		written = s
		return nil
	})

	bodyJSON := `{"text": "Text from mobile 📱"}`
	req := httptest.NewRequest("POST", "/api/clipboard", strings.NewReader(bodyJSON))
	req.Host = "127.0.0.1:8765"
	req.RemoteAddr = "127.0.0.1:50000"
	req.Header.Set("Authorization", "Bearer secret-token")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.handleClipboard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	if written != "Text from mobile 📱" {
		t.Errorf("expected written text 'Text from mobile 📱', got %q", written)
	}

	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if res["status"] != "success" {
		t.Errorf("expected status 'success', got %v", res["status"])
	}
}

func TestServerClipboardPostRawText(t *testing.T) {
	mockCap := &mockCapturer{img: createTestImage(10, 10)}
	srv, _ := setupTestServer(mockCap, "secret-token")

	var written string
	srv.SetClipboardFuncs(nil, func(s string) error {
		written = s
		return nil
	})

	raw := "Just a raw string payload without JSON wrappers"
	req := httptest.NewRequest("POST", "/api/clipboard", strings.NewReader(raw))
	req.Host = "127.0.0.1:8765"
	req.RemoteAddr = "127.0.0.1:50000"
	req.Header.Set("Authorization", "Bearer secret-token")

	w := httptest.NewRecorder()
	srv.handleClipboard(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	if written != raw {
		t.Errorf("expected %q, got %q", raw, written)
	}
}

func TestServerClipboardErrors(t *testing.T) {
	mockCap := &mockCapturer{img: createTestImage(10, 10)}
	srv, _ := setupTestServer(mockCap, "secret-token")

	srv.SetClipboardFuncs(func() (string, error) {
		return "", errors.New("read failed")
	}, func(s string) error {
		return errors.New("write failed")
	})

	// 1. GET read failure
	reqGet := httptest.NewRequest("GET", "/api/clipboard", nil)
	reqGet.Host = "127.0.0.1:8765"
	reqGet.RemoteAddr = "127.0.0.1:50000"
	reqGet.Header.Set("Authorization", "Bearer secret-token")

	wGet := httptest.NewRecorder()
	srv.handleClipboard(wGet, reqGet)

	if wGet.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on read failure, got %d", wGet.Code)
	}

	// 2. POST write failure
	reqPost := httptest.NewRequest("POST", "/api/clipboard", strings.NewReader(`{"text":"hello"}`))
	reqPost.Host = "127.0.0.1:8765"
	reqPost.RemoteAddr = "127.0.0.1:50000"
	reqPost.Header.Set("Authorization", "Bearer secret-token")

	wPost := httptest.NewRecorder()
	srv.handleClipboard(wPost, reqPost)

	if wPost.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on write failure, got %d", wPost.Code)
	}
}