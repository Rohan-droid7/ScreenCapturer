package server

import (
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"

	"ScreenCapturer/internal/auth"
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
	authMgr := auth.NewManager(func() string { return cfg.Token }, cfg.AllowedHosts, cfg.Port)
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
