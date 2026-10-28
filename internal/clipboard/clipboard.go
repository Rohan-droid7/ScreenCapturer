package clipboard

import (
	"errors"
)

var (
	// ErrNoText is returned when the clipboard does not contain text data.
	ErrNoText = errors.New("clipboard does not contain text")
)

// Content wraps clipboard text data and metadata.
type Content struct {
	Text    string `json:"text"`
	Length  int    `json:"length"`
	HasText bool   `json:"has_text"`
}
