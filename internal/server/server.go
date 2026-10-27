package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"ScreenCapturer/internal/auth"
	"ScreenCapturer/internal/capture"
	"ScreenCapturer/internal/config"
	"ScreenCapturer/internal/logger"
	"ScreenCapturer/internal/tunnel"
	"ScreenCapturer/web"
)

// Server wraps the HTTP listener and handlers.
type Server struct {
	cfg          *config.Config
	authMgr      *auth.Manager
	capturer     capture.Capturer
	tunnelMgr    *tunnel.Manager
	httpServer   *http.Server
	listener     net.Listener
	captureMu    sync.Mutex
	running      bool
	mu           sync.Mutex
	onCaptureCb  func() // Optional callback when a capture succeeds
	shutdownChan chan struct{}
	shutdownOnce sync.Once
	lockFunc     func() error
}

// NewServer constructs the HTTP server with timeouts and routes.
// tunnelMgr is optional and may be passed as an optional 4th parameter.
func NewServer(cfg *config.Config, capturer capture.Capturer, authMgr *auth.Manager, tunnelMgr ...*tunnel.Manager) *Server {
	var tm *tunnel.Manager
	if len(tunnelMgr) > 0 {
		tm = tunnelMgr[0]
	}
	s := &Server{
		cfg:          cfg,
		capturer:     capturer,
		authMgr:      authMgr,
		tunnelMgr:    tm,
		shutdownChan: make(chan struct{}),
		lockFunc:     LockPC,
	}
	return s
}

// WaitForShutdown blocks until the server receives a shutdown request.
func (s *Server) WaitForShutdown() {
	<-s.shutdownChan
}

// TriggerShutdown signals the server to terminate.
func (s *Server) TriggerShutdown() {
	s.shutdownOnce.Do(func() {
		close(s.shutdownChan)
	})
}

// SetOnCaptureCallback registers a hook called whenever an authenticated capture completes.
func (s *Server) SetOnCaptureCallback(cb func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCaptureCb = cb
}

// SetLockFunc overrides the workstation lock handler (useful for testing or platform customization).
func (s *Server) SetLockFunc(fn func() error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lockFunc = fn
}

// GetPrimaryLANIP returns the first non-loopback private IPv4 address.
func GetPrimaryLANIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
				return ip.String()
			}
		}
	}
	return "127.0.0.1"
}

func enableCORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

// Start binds the port and starts listening for HTTP requests.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("server is already running")
	}

	mux := http.NewServeMux()

	// Web interface and static assets
	mux.Handle("/", web.Handler())

	// API endpoints
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/capture", s.handleCapture)
	mux.HandleFunc("/api/lock", s.handleLockPC)
	mux.HandleFunc("/api/shutdown", s.handleShutdown)
	mux.HandleFunc("/api/tunnel/status", s.handleTunnelStatus)
	mux.HandleFunc("/api/tunnel/toggle", s.handleTunnelToggle)
	mux.HandleFunc("/pairing", s.handlePairingPage)

	addr := fmt.Sprintf("0.0.0.0:%d", s.cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		if strings.Contains(err.Error(), "bind: Only one usage") || strings.Contains(err.Error(), "already in use") {
			return fmt.Errorf("port %d is already in use by another application: %w", s.cfg.Port, err)
		}
		return fmt.Errorf("failed to bind on %s: %w", addr, err)
	}

	s.listener = listener
	s.httpServer = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	s.running = true
	s.mu.Unlock()

	logger.Get().Infof("ScreenCapturer server listening on %s (LAN: %s)", addr, GetPrimaryLANIP())

	go func() {
		if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Get().Errorf("HTTP serve error: %v", err)
		}
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	return nil
}

func (s *Server) handlePairingPage(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	html := GeneratePairingHTML(s.cfg, s.tunnelMgr)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

// Stop gracefully shuts down the server.
func (s *Server) Stop() error {
	s.TriggerShutdown()
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tunnelMgr != nil {
		_ = s.tunnelMgr.Stop()
	}

	if !s.running || s.httpServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := s.httpServer.Shutdown(ctx)
	s.running = false
	logger.Get().Infof("ScreenCapturer server stopped")
	return err
}

// IsRunning reports if the server is actively serving.
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := map[string]any{
		"status":         "running",
		"engine":         s.capturer.Name(),
		"port":           s.cfg.Port,
		"lan_ip":         GetPrimaryLANIP(),
		"default_format": s.cfg.DefaultFormat,
	}

	if s.tunnelMgr != nil {
		tStatus := s.tunnelMgr.GetStatus()
		resp["tunnel_active"] = tStatus.Active
		resp["tunnel_connecting"] = tStatus.Connecting
		resp["tunnel_url"] = tStatus.PublicURL
		if tStatus.Error != "" {
			resp["tunnel_error"] = tStatus.Error
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleTunnelStatus(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if s.tunnelMgr == nil {
		_ = json.NewEncoder(w).Encode(tunnel.Status{Active: false, Error: "Tunnel manager not initialized"})
		return
	}

	_ = json.NewEncoder(w).Encode(s.tunnelMgr.GetStatus())
}

func (s *Server) handleTunnelToggle(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed; use POST", http.StatusMethodNotAllowed)
		return
	}

	// Verify authentication
	if err := s.authMgr.CheckRequest(r); err != nil {
		sendJSONError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error(), "UNAUTHORIZED")
		return
	}

	if s.tunnelMgr == nil {
		sendJSONError(w, http.StatusBadRequest, "Tunnel manager not enabled", "TUNNEL_DISABLED")
		return
	}

	active, err := s.tunnelMgr.Toggle()
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error(), "TUNNEL_ERROR")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"active":     active,
		"connecting": s.tunnelMgr.GetStatus().Connecting,
		"public_url": s.tunnelMgr.GetPublicURL(),
	})
}

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

func sendJSONError(w http.ResponseWriter, status int, msg string, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: msg,
		Code:  code,
	})
}

func (s *Server) handleCapture(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	startTime := time.Now()
	clientIP := auth.CleanIP(r.RemoteAddr)

	if r.Method != http.MethodPost {
		sendJSONError(w, http.StatusMethodNotAllowed, "Method not allowed; use POST", "METHOD_NOT_ALLOWED")
		return
	}

	// Limit request body
	r.Body = http.MaxBytesReader(w, r.Body, 1024)

	// Authenticate request & check rate limits / lockout
	if err := s.authMgr.CheckRequest(r); err != nil {
		if errors.Is(err, auth.ErrLockedOut) {
			sendJSONError(w, http.StatusTooManyRequests, err.Error(), "IP_LOCKED_OUT")
			return
		}
		if errors.Is(err, auth.ErrInvalidHost) {
			sendJSONError(w, http.StatusBadRequest, "Invalid or unauthorized Host header", "INVALID_HOST")
			return
		}
		sendJSONError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error(), "UNAUTHORIZED")
		return
	}

	// Format selection (?format=png or ?format=jpeg)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = s.cfg.DefaultFormat
		if format == "" {
			format = "png"
		}
	}

	// Serialize screen capture operations to protect memory
	s.captureMu.Lock()
	defer s.captureMu.Unlock()

	// Capture display with one retry
	capStart := time.Now()
	img, capErr := capture.CaptureWithRetry(s.capturer)
	captureDur := time.Since(capStart)

	if capErr != nil {
		logger.Get().Errorf("Screen capture failed for %s: %v", clientIP, capErr)

		if errors.Is(capErr, capture.ErrScreenLocked) {
			sendJSONError(w, http.StatusServiceUnavailable, "Screen is currently locked or secure desktop is active", "SCREEN_LOCKED")
			return
		}
		if errors.Is(capErr, capture.ErrDisplayUnavailable) {
			sendJSONError(w, http.StatusServiceUnavailable, "Display device is unavailable or sleeping", "DISPLAY_UNAVAILABLE")
			return
		}
		sendJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Screen capture failed: %v", capErr), "CAPTURE_FAILED")
		return
	}

	// Encode image to buffer first to know content length and catch encoding errors before headers
	encStart := time.Now()
	var buf bytes.Buffer
	mimeType, encErr := EncodeImage(&buf, img, format, s.cfg.JPEGQuality)
	encodeDur := time.Since(encStart)

	if encErr != nil {
		logger.Get().Errorf("Encoding failed: %v", encErr)
		sendJSONError(w, http.StatusBadRequest, encErr.Error(), "ENCODE_ERROR")
		return
	}

	// Release pixel buffer from img and free OS memory
	img = nil
	debug.FreeOSMemory()

	totalDur := time.Since(startTime)
	encodedBytes := buf.Len()

	// Log timings without sensitive data
	logger.Get().LogCaptureTiming(clientIP, s.capturer.Name(), format, captureDur, encodeDur, totalDur, encodedBytes)

	// Send image response
	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", encodedBytes))
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())

	// Notify callback for tray/toast
	s.mu.Lock()
	cb := s.onCaptureCb
	s.mu.Unlock()
	if cb != nil {
		go cb()
	}
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		sendJSONError(w, http.StatusMethodNotAllowed, "Method not allowed; use POST", "METHOD_NOT_ALLOWED")
		return
	}

	// Authenticate request (bearer token + host validation)
	if err := s.authMgr.CheckRequest(r); err != nil {
		sendJSONError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error(), "UNAUTHORIZED")
		return
	}

	logger.Get().Infof("Graceful shutdown requested from %s. Exiting...", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "stopped",
		"message": "ScreenCapturer has shut down successfully.",
	})

	// Terminate process after short delay so the response finishes sending
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = s.Stop()
	}()
}

func (s *Server) handleLockPC(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		sendJSONError(w, http.StatusMethodNotAllowed, "Method not allowed; use POST", "METHOD_NOT_ALLOWED")
		return
	}

	// Limit request body
	r.Body = http.MaxBytesReader(w, r.Body, 1024)

	// Authenticate request & check rate limits / lockout
	if err := s.authMgr.CheckRequest(r); err != nil {
		if errors.Is(err, auth.ErrLockedOut) {
			sendJSONError(w, http.StatusTooManyRequests, err.Error(), "IP_LOCKED_OUT")
			return
		}
		if errors.Is(err, auth.ErrInvalidHost) {
			sendJSONError(w, http.StatusBadRequest, "Invalid or unauthorized Host header", "INVALID_HOST")
			return
		}
		sendJSONError(w, http.StatusUnauthorized, "Unauthorized: "+err.Error(), "UNAUTHORIZED")
		return
	}

	logger.Get().Infof("Remote workstation lock requested from %s", auth.CleanIP(r.RemoteAddr))

	s.mu.Lock()
	lockFn := s.lockFunc
	s.mu.Unlock()

	if lockFn == nil {
		lockFn = LockPC
	}

	if err := lockFn(); err != nil {
		logger.Get().Errorf("Workstation lock failed: %v", err)
		sendJSONError(w, http.StatusInternalServerError, "Failed to lock workstation: "+err.Error(), "LOCK_FAILED")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "locked",
		"message": "Workstation locked successfully.",
	})
}