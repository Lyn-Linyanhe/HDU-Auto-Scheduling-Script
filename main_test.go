package main

import (
	neturl "net/url"
	"testing"
)

func TestAllowedLocalOrigin(t *testing.T) {
	tests := []struct {
		origin string
		port   string
		want   bool
	}{
		{"http://127.0.0.1:6789", "6789", true},
		{"http://localhost:6789", "6789", true},
		{"http://[::1]:6789", "6789", true},
		{"http://127.0.0.1:6790", "6789", false},
		{"https://127.0.0.1:6789", "6789", false},
		{"http://example.com:6789", "6789", false},
	}

	for _, tt := range tests {
		parsed, err := neturl.Parse(tt.origin)
		if err != nil {
			t.Fatalf("parse origin %q: %v", tt.origin, err)
		}
		if got := isAllowedLocalOrigin(parsed, tt.port); got != tt.want {
			t.Fatalf("isAllowedLocalOrigin(%q, %q) = %v, want %v", tt.origin, tt.port, got, tt.want)
		}
	}
}
