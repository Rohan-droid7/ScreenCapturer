package tunnel

import (
	"testing"
)

func TestTunnelURLRegex(t *testing.T) {
	sampleOutputs := []struct {
		line     string
		expected string
	}{
		{
			line:     "2026-10-25T16:00:00Z INF +--------------------------------------------------------------------------------------------+",
			expected: "",
		},
		{
			line:     "2026-10-25T16:00:00Z INF |  Your quick Tunnel has been created! Visit it at (it may take some time to be reachable):  |",
			expected: "",
		},
		{
			line:     "2026-10-25T16:00:00Z INF |  https://purple-falcon-flight-sample.trycloudflare.com                                    |",
			expected: "https://purple-falcon-flight-sample.trycloudflare.com",
		},
		{
			line:     "Visit https://random-1234-xyz.trycloudflare.com for preview",
			expected: "https://random-1234-xyz.trycloudflare.com",
		},
	}

	for _, tt := range sampleOutputs {
		match := tunnelURLRegex.FindString(tt.line)
		if match != tt.expected {
			t.Errorf("tunnelURLRegex.FindString(%q) = %q; want %q", tt.line, match, tt.expected)
		}
	}
}

func TestManagerInitialState(t *testing.T) {
	mgr := NewManager(8765)
	if mgr.IsActive() {
		t.Errorf("expected manager to be inactive initially")
	}
	if mgr.GetPublicURL() != "" {
		t.Errorf("expected empty public URL initially")
	}

	status := mgr.GetStatus()
	if status.Active {
		t.Errorf("status.Active should be false")
	}
	if status.Connecting {
		t.Errorf("status.Connecting should be false")
	}
	if status.PublicURL != "" {
		t.Errorf("status.PublicURL should be empty")
	}

	// Stop when already inactive should not panic or error
	if err := mgr.Stop(); err != nil {
		t.Errorf("Stop() on inactive manager returned error: %v", err)
	}
}
