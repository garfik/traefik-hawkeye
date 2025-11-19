package traefik_hawkeye

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		remote   string
		expected string
	}{
		{
			name:     "X-Real-Ip header",
			headers:  map[string]string{"X-Real-Ip": "192.168.1.1"},
			remote:   "10.0.0.1:12345",
			expected: "192.168.1.1",
		},
		{
			name:     "X-Real-Ip header lowercase",
			headers:  map[string]string{"x-real-ip": "192.168.1.1"},
			remote:   "10.0.0.1:12345",
			expected: "192.168.1.1",
		},
		{
			name: "Both X-Real-Ip and X-Forwarded-For",
			headers: map[string]string{
				"X-Real-Ip":       "203.0.113.1",
				"X-Forwarded-For": "192.168.1.2, 10.0.0.1",
			},
			remote:   "10.0.0.1:12345",
			expected: "203.0.113.1",
		},
		{
			name:     "X-Forwarded-For header",
			headers:  map[string]string{"X-Forwarded-For": "192.168.1.2, 10.0.0.1"},
			remote:   "10.0.0.1:12345",
			expected: "192.168.1.2",
		},
		{
			name:     "RemoteAddr fallback",
			headers:  map[string]string{},
			remote:   "192.168.1.3:12345",
			expected: "192.168.1.3",
		},
		{
			name:     "RemoteAddr without port",
			headers:  map[string]string{},
			remote:   "192.168.1.4",
			expected: "192.168.1.4",
		},
		{
			name:     "X-Forwarded-For uppercase value and spaces",
			headers:  map[string]string{"X-FORWARDED-FOR": "  198.51.100.5  , 10.0.0.1"},
			remote:   "10.0.0.1:12345",
			expected: "198.51.100.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tt.remote

			result := extractIP(req)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestExtractScheme(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		tls      bool
		expected string
	}{
		{
			name:     "X-Forwarded-Proto header https",
			headers:  map[string]string{"X-Forwarded-Proto": "https"},
			tls:      false,
			expected: "https",
		},
		{
			name:     "X-Forwarded-Proto header http",
			headers:  map[string]string{"X-Forwarded-Proto": "http"},
			tls:      false,
			expected: "http",
		},
		{
			name:     "X-Forwarded-Proto uppercase value",
			headers:  map[string]string{"X-FORWARDED-PROTO": "HTTPS"},
			tls:      false,
			expected: "https",
		},
		{
			name: "Header takes precedence over TLS",
			headers: map[string]string{
				"X-Forwarded-Proto": "http",
			},
			tls:      true,
			expected: "http",
		},
		{
			name:     "TLS present",
			headers:  map[string]string{},
			tls:      true,
			expected: "https",
		},
		{
			name:     "Default http",
			headers:  map[string]string{},
			tls:      false,
			expected: "http",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}

			result := extractScheme(req)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestNormalizeContentType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple type",
			input:    "application/json",
			expected: "application/json",
		},
		{
			name:     "with charset",
			input:    "text/html; charset=utf-8",
			expected: "text/html",
		},
		{
			name:     "with multiple parameters",
			input:    "application/json; charset=utf-8; boundary=something",
			expected: "application/json",
		},
		{
			name:     "uppercase",
			input:    "APPLICATION/JSON",
			expected: "application/json",
		},
		{
			name:     "mixed case",
			input:    "Text/HTML; Charset=UTF-8",
			expected: "text/html",
		},
		{
			name:     "with spaces",
			input:    "  text/html  ;  charset=utf-8  ",
			expected: "text/html",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only spaces",
			input:    "   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeContentType(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestCopyHeaders(t *testing.T) {
	src := http.Header{}
	src.Set("X-Test", "value1")
	src.Add("X-Test", "value2")
	src.Set("X-Other", "alpha")

	dst := http.Header{}
	copyHeaders(dst, src)

	if !reflect.DeepEqual(dst, src) {
		t.Fatalf("expected dst to equal src, got dst=%v src=%v", dst, src)
	}

	src.Add("X-Test", "value3")
	if len(dst["X-Test"]) != 2 {
		t.Fatalf("expected dst slice to remain unchanged, got %v", dst["X-Test"])
	}
}

func TestIsWebSocketUpgrade(t *testing.T) {
	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name:     "Valid WebSocket upgrade",
			headers:  map[string]string{"Upgrade": "websocket", "Connection": "Upgrade"},
			expected: true,
		},
		{
			name:     "Valid WebSocket upgrade lowercase",
			headers:  map[string]string{"Upgrade": "WebSocket", "Connection": "upgrade"},
			expected: true,
		},
		{
			name:     "Valid WebSocket upgrade with multiple connection values",
			headers:  map[string]string{"Upgrade": "websocket", "Connection": "keep-alive, Upgrade"},
			expected: true,
		},
		{
			name:     "No upgrade header",
			headers:  map[string]string{"Connection": "Upgrade"},
			expected: false,
		},
		{
			name:     "No connection header",
			headers:  map[string]string{"Upgrade": "websocket"},
			expected: false,
		},
		{
			name:     "Wrong upgrade value",
			headers:  map[string]string{"Upgrade": "http/2.0", "Connection": "Upgrade"},
			expected: false,
		},
		{
			name:     "No headers",
			headers:  map[string]string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := isWebSocketUpgrade(req)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
