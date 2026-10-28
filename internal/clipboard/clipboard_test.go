package clipboard

import (
	"testing"
)

func TestClipboardRoundtrip(t *testing.T) {
	// Backup existing clipboard content
	orig, _ := Read()

	testStrings := []string{
		"Hello World",
		"Special chars: !@#$%^&*()_+-=[]{}|;':\",./<>?",
		"Unicode and Emojis: 🚀🔥💻 Quick Brown Fox 🦊",
		"Multi-line:\nLine 1\nLine 2\r\nLine 3\n\tIndented",
	}

	for _, text := range testStrings {
		if err := Write(text); err != nil {
			t.Fatalf("Write(%q) failed: %v", text, err)
		}

		got, err := Read()
		if err != nil {
			t.Fatalf("Read() after Write(%q) failed: %v", text, err)
		}

		if got != text {
			t.Errorf("Clipboard mismatch: got %q, want %q", got, text)
		}
	}

	// Restore original clipboard
	if orig != "" {
		_ = Write(orig)
	}
}
