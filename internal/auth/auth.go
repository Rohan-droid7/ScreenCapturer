package auth

import (
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	ErrLockedOut      = errors.New("too many failed authentication attempts; client locked out for 60s")
	ErrMissingAuth    = errors.New("missing or malformed Authorization header")
	ErrInvalidToken   = errors.New("invalid authorization token")
	ErrInvalidHost    = errors.New("invalid or unauthorized Host header")
)

type ipRecord struct {
	failures    int
	lockedUntil time.Time
}

// Manager coordinates token verification, host validation, and rate limiting / IP lockout.
type Manager struct {
	tokenGetter  func() string
	allowedHosts []string
	port         int
	nowFn        func() time.Time // for deterministic unit tests

	mu          sync.Mutex
	ipRecords   map[string]*ipRecord
	lockoutDur  time.Duration
	maxFailures int
}

// NewManager creates an authentication manager with 5 max failures and 60s lockout.
func NewManager(tokenGetter func() string, allowedHosts []string, port int) *Manager {
	return &Manager{
		tokenGetter:  tokenGetter,
		allowedHosts: allowedHosts,
		port:         port,
		nowFn:        time.Now,
		ipRecords:    make(map[string]*ipRecord),
		lockoutDur:   60 * time.Second,
		maxFailures:  5,
	}
}

// SetTimeFunc sets custom clock (useful for unit tests).
func (m *Manager) SetTimeFunc(fn func() time.Time) {
	m.nowFn = fn
}

// CleanIP extracts the IP address from a host:port RemoteAddr string.
func CleanIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	// Fallback if no port
	return remoteAddr
}

// ValidateHost verifies that the Host header matches allowed hosts or local interfaces.
func (m *Manager) ValidateHost(hostHeader string) bool {
	if hostHeader == "" {
		return false
	}

	host, _, err := net.SplitHostPort(hostHeader)
	if err != nil {
		host = hostHeader
	}
	host = strings.TrimSpace(strings.ToLower(host))

	// Always allow loopback
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}

	for _, allowed := range m.allowedHosts {
		if strings.EqualFold(host, strings.ToLower(allowed)) {
			return true
		}
	}

	// Check if host is a valid IP on any local network interface
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() {
			return true
		}
	}

	return false
}

// VerifyToken compares a bearer token using crypto/subtle to prevent timing attacks.
func (m *Manager) VerifyToken(token string) bool {
	expected := m.tokenGetter()
	if expected == "" || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

// CheckRequest validates the Host header and the Bearer token, enforcing the 5-attempt / 60s lockout.
func (m *Manager) CheckRequest(r *http.Request) error {
	// First validate host
	if !m.ValidateHost(r.Host) {
		return ErrInvalidHost
	}

	ip := CleanIP(r.RemoteAddr)
	now := m.nowFn()

	m.mu.Lock()
	defer m.mu.Unlock()

	rec, exists := m.ipRecords[ip]
	if !exists {
		rec = &ipRecord{}
		m.ipRecords[ip] = rec
	}

	// Check if client is currently locked out
	if now.Before(rec.lockedUntil) {
		return ErrLockedOut
	}

	// Extract Bearer token
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		m.recordFailureLocked(rec, now)
		return ErrMissingAuth
	}

	providedToken := strings.TrimSpace(authHeader[7:])
	if !m.VerifyToken(providedToken) {
		m.recordFailureLocked(rec, now)
		return ErrInvalidToken
	}

	// Successful auth resets failure counter
	rec.failures = 0
	rec.lockedUntil = time.Time{}
	return nil
}

func (m *Manager) recordFailureLocked(rec *ipRecord, now time.Time) {
	rec.failures++
	if rec.failures >= m.maxFailures {
		rec.lockedUntil = now.Add(m.lockoutDur)
		rec.failures = 0 // reset failure count for next lockout cycle
	}
}

// IsLockedOut checks if an IP is currently locked out.
func (m *Manager) IsLockedOut(ip string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, exists := m.ipRecords[ip]
	if !exists {
		return false
	}
	return m.nowFn().Before(rec.lockedUntil)
}
