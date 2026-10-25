package auth

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidateHost(t *testing.T) {
	mgr := NewManager(func() string { return "test-token" }, []string{"laptop.local", "my-pc"}, 8765)

	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		{"empty host", "", false},
		{"localhost with port", "localhost:8765", true},
		{"localhost without port", "localhost", true},
		{"127.0.0.1", "127.0.0.1:8765", true},
		{"IPv6 loopback", "[::1]:8765", true},
		{"explicit allowed host", "laptop.local:8765", true},
		{"explicit allowed host case-insensitive", "LAPTOP.LOCAL", true},
		{"private LAN IP 192.168.1.50", "192.168.1.50:8765", true},
		{"private LAN IP 10.0.0.5", "10.0.0.5:8765", true},
		{"private LAN IP 172.16.0.2", "172.16.0.2:8765", true},
		{"public internet IP rejected", "8.8.8.8:8765", false},
		{"arbitrary external domain rejected", "evil-attacker.com", false},
		{"arbitrary external domain with port rejected", "evil.com:8765", false},
		{"cloudflare quick tunnel subdomain", "random-words-123.trycloudflare.com", true},
		{"cloudflare quick tunnel with port", "random-words-123.trycloudflare.com:443", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mgr.ValidateHost(tt.host)
			if got != tt.expected {
				t.Errorf("ValidateHost(%q) = %v; want %v", tt.host, got, tt.expected)
			}
		})
	}

	// Test dynamically added allowed host
	if mgr.ValidateHost("custom-domain.net") {
		t.Errorf("expected custom-domain.net to be rejected initially")
	}
	mgr.AddAllowedHost("custom-domain.net")
	if !mgr.ValidateHost("custom-domain.net:8765") {
		t.Errorf("expected custom-domain.net to be accepted after AddAllowedHost")
	}
}

func TestVerifyToken(t *testing.T) {
	validToken := "e2d88198f3b259d64f891b65e921d7b67b120c24c29759ff539e93175c5e884b"
	mgr := NewManager(func() string { return validToken }, nil, 8765)

	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{"matching token", validToken, true},
		{"empty token", "", false},
		{"wrong token", "wrong-token-abc", false},
		{"one char different", validToken[:len(validToken)-1] + "a", false},
		{"prefix only", validToken[:10], false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mgr.VerifyToken(tt.token)
			if got != tt.expected {
				t.Errorf("VerifyToken(%q) = %v; want %v", tt.token, got, tt.expected)
			}
		})
	}
}

func TestLockoutAfter5Failures(t *testing.T) {
	validToken := "correct-secret-token"
	mgr := NewManager(func() string { return validToken }, nil, 8765)

	currentTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	mgr.SetTimeFunc(func() time.Time {
		return currentTime
	})

	remoteAddr := "192.168.1.100:54321"

	// 4 failed attempts should fail with ErrInvalidToken, NOT locked out yet
	for i := 1; i <= 4; i++ {
		req := httptest.NewRequest("POST", "/api/capture", nil)
		req.Host = "192.168.1.100:8765"
		req.RemoteAddr = remoteAddr
		req.Header.Set("Authorization", "Bearer wrong-token")

		err := mgr.CheckRequest(req)
		if err != ErrInvalidToken {
			t.Fatalf("attempt %d: expected ErrInvalidToken, got %v", i, err)
		}
		if mgr.IsLockedOut(CleanIP(remoteAddr)) {
			t.Fatalf("attempt %d: client should not be locked out yet", i)
		}
	}

	// 5th failed attempt triggers the lockout
	req5 := httptest.NewRequest("POST", "/api/capture", nil)
	req5.Host = "192.168.1.100:8765"
	req5.RemoteAddr = remoteAddr
	req5.Header.Set("Authorization", "Bearer wrong-token")

	err5 := mgr.CheckRequest(req5)
	if err5 != ErrInvalidToken {
		t.Fatalf("attempt 5: expected ErrInvalidToken, got %v", err5)
	}
	if !mgr.IsLockedOut(CleanIP(remoteAddr)) {
		t.Fatalf("attempt 5: client should be locked out now")
	}

	// 6th attempt (even with CORRECT token) during lockout period must be rejected with ErrLockedOut
	req6 := httptest.NewRequest("POST", "/api/capture", nil)
	req6.Host = "192.168.1.100:8765"
	req6.RemoteAddr = remoteAddr
	req6.Header.Set("Authorization", "Bearer "+validToken)

	err6 := mgr.CheckRequest(req6)
	if err6 != ErrLockedOut {
		t.Fatalf("attempt 6 during lockout: expected ErrLockedOut, got %v", err6)
	}

	// Fast forward 61 seconds (lockout period expires)
	currentTime = currentTime.Add(61 * time.Second)

	// Now correct token should succeed
	reqSuccess := httptest.NewRequest("POST", "/api/capture", nil)
	reqSuccess.Host = "192.168.1.100:8765"
	reqSuccess.RemoteAddr = remoteAddr
	reqSuccess.Header.Set("Authorization", "Bearer "+validToken)

	errSuccess := mgr.CheckRequest(reqSuccess)
	if errSuccess != nil {
		t.Fatalf("expected successful auth after lockout expired, got %v", errSuccess)
	}

	// Lockout status should now be clear
	if mgr.IsLockedOut(CleanIP(remoteAddr)) {
		t.Fatalf("client should no longer be locked out after successful auth")
	}
}
