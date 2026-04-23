package pagination

import (
	"encoding/base64"
	"fmt"
	"strconv"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
	MinLimit     = 1
)

// Params holds parsed pagination parameters.
type Params struct {
	cursor string
	Limit  int
}

// ParseParams validates and constructs pagination parameters from raw query values.
func ParseParams(cursor, limitStr string) (Params, error) {
	limit := DefaultLimit
	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < MinLimit || l > MaxLimit {
			return Params{}, fmt.Errorf("limit must be between %d and %d", MinLimit, MaxLimit)
		}
		limit = l
	}

	if cursor != "" {
		decoded, err := DecodeCursor(cursor)
		if err != nil {
			return Params{}, fmt.Errorf("invalid cursor: %w", err)
		}
		return Params{cursor: decoded, Limit: limit}, nil
	}

	return Params{Limit: limit}, nil
}

// FetchLimit returns limit+1 for has_more detection.
func (p Params) FetchLimit() int {
	return p.Limit + 1
}

// CursorID returns the decoded cursor ID or empty string for the first page.
func (p Params) CursorID() string {
	return p.cursor
}

// EncodeCursor base64-encodes an ID into an opaque cursor token.
func EncodeCursor(id string) string {
	return base64.URLEncoding.EncodeToString([]byte(id))
}

// DecodeCursor decodes a base64 cursor token back to an ID.
func DecodeCursor(cursor string) (string, error) {
	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return "", fmt.Errorf("decoding cursor: %w", err)
	}
	return string(decoded), nil
}
