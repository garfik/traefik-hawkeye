package traefik_hawkeye

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFilter_ShouldWrap(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		req      *http.Request
		expected bool
	}{
		{
			name:     "WebSocket request should not wrap",
			config:   CreateConfig(),
			req:      createWebSocketRequest(),
			expected: false,
		},
		{
			name:     "Regular request should wrap",
			config:   CreateConfig(),
			req:      httptest.NewRequest(http.MethodGet, "http://example.com", nil),
			expected: true,
		},
		{
			name: "Host filter exclude - host in list should not wrap",
			config: &Config{
				FilterHostMode: "exclude",
				FilterHostList: []string{"skip.com"},
			},
			req:      createRequestWithHost("skip.com"),
			expected: false,
		},
		{
			name: "Host filter exclude - host not in list should wrap",
			config: &Config{
				FilterHostMode: "exclude",
				FilterHostList: []string{"skip.com"},
			},
			req:      createRequestWithHost("example.com"),
			expected: true,
		},
		{
			name: "Host filter include - host in list should wrap",
			config: &Config{
				FilterHostMode: "include",
				FilterHostList: []string{"example.com"},
			},
			req:      createRequestWithHost("example.com"),
			expected: true,
		},
		{
			name: "Host filter include - uppercase request host matches",
			config: &Config{
				FilterHostMode: "include",
				FilterHostList: []string{"example.com"},
			},
			req:      createRequestWithHost("EXAMPLE.COM"),
			expected: true,
		},
		{
			name: "Host filter include - host not in list should not wrap",
			config: &Config{
				FilterHostMode: "include",
				FilterHostList: []string{"example.com"},
			},
			req:      createRequestWithHost("other.com"),
			expected: false,
		},
		{
			name: "No host filter - should wrap",
			config: &Config{
				FilterHostMode: "",
				FilterHostList: []string{},
			},
			req:      createRequestWithHost("example.com"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewFilter(tt.config)
			result := filter.ShouldWrap(tt.req)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFilter_ShouldLog(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		req         *http.Request
		contentType string
		expected    bool
	}{
		{
			name:        "No filter - should log",
			config:      CreateConfig(),
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/test", nil),
			contentType: "application/json",
			expected:    true,
		},
		{
			name: "ContentType filter include - type in list should log",
			config: &Config{
				FilterContentTypeMode: "include",
				FilterContentTypeList: []string{"application/json"},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/test", nil),
			contentType: "application/json",
			expected:    true,
		},
		{
			name: "ContentType filter include - type not in list should not log",
			config: &Config{
				FilterContentTypeMode: "include",
				FilterContentTypeList: []string{"application/json"},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/test", nil),
			contentType: "text/html",
			expected:    false,
		},
		{
			name: "ContentType filter exclude - type in list should not log",
			config: &Config{
				FilterContentTypeMode: "exclude",
				FilterContentTypeList: []string{"text/html"},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/test", nil),
			contentType: "text/html",
			expected:    false,
		},
		{
			name: "ContentType filter exclude - type not in list should log",
			config: &Config{
				FilterContentTypeMode: "exclude",
				FilterContentTypeList: []string{"text/html"},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/test", nil),
			contentType: "application/json",
			expected:    true,
		},
		{
			name: "ContentType filter with charset - should normalize",
			config: &Config{
				FilterContentTypeMode: "include",
				FilterContentTypeList: []string{"application/json"},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/test", nil),
			contentType: "application/json; charset=utf-8",
			expected:    true,
		},
		{
			name: "ContentType filter case insensitive",
			config: &Config{
				FilterContentTypeMode: "include",
				FilterContentTypeList: []string{"application/json"},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/test", nil),
			contentType: "APPLICATION/JSON",
			expected:    true,
		},
		{
			name: "ContentType filter empty list - should log",
			config: &Config{
				FilterContentTypeMode: "include",
				FilterContentTypeList: []string{},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/test", nil),
			contentType: "application/json",
			expected:    true,
		},
		{
			name: "ContentType filter empty mode - should log",
			config: &Config{
				FilterContentTypeMode: "",
				FilterContentTypeList: []string{"application/json"},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/test", nil),
			contentType: "application/json",
			expected:    true,
		},
		{
			name: "ContentType filter empty content type",
			config: &Config{
				FilterContentTypeMode: "include",
				FilterContentTypeList: []string{"application/json"},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/test", nil),
			contentType: "",
			expected:    false,
		},
		{
			name: "Tracking pixel with valid u - should log (bypasses content type filter)",
			config: &Config{
				TrackingPixelURL:      "/hawk.png",
				FilterContentTypeMode: "exclude",
				FilterContentTypeList: []string{"image/png"},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/hawk.png?u=%2F", nil),
			contentType: "image/png",
			expected:    true,
		},
		{
			name: "Tracking pixel without u - should apply normal filter",
			config: &Config{
				TrackingPixelURL:      "/hawk.png",
				FilterContentTypeMode: "exclude",
				FilterContentTypeList: []string{"image/png"},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/hawk.png", nil),
			contentType: "image/png",
			expected:    false,
		},
		{
			name: "Tracking pixel with invalid u - should apply normal filter",
			config: &Config{
				TrackingPixelURL:      "/hawk.png",
				FilterContentTypeMode: "exclude",
				FilterContentTypeList: []string{"image/png"},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/hawk.png?u=%ZZ", nil),
			contentType: "image/png",
			expected:    false,
		},
		{
			name: "Tracking pixel different path - should apply normal filter",
			config: &Config{
				TrackingPixelURL:      "/hawk.png",
				FilterContentTypeMode: "exclude",
				FilterContentTypeList: []string{"image/png"},
			},
			req:         httptest.NewRequest(http.MethodGet, "http://example.com/other.png?u=%2Fmap%2Ftest", nil),
			contentType: "image/png",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewFilter(tt.config)
			result := filter.ShouldLog(tt.req, tt.contentType)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFilter_ShouldLogHost(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		host     string
		expected bool
	}{
		{
			name: "No filter - should log",
			config: &Config{
				FilterHostMode: "",
				FilterHostList: []string{},
			},
			host:     "example.com",
			expected: true,
		},
		{
			name: "Include mode - host in list",
			config: &Config{
				FilterHostMode: "include",
				FilterHostList: []string{"example.com", "test.com"},
			},
			host:     "example.com",
			expected: true,
		},
		{
			name: "Include mode - host not in list",
			config: &Config{
				FilterHostMode: "include",
				FilterHostList: []string{"example.com", "test.com"},
			},
			host:     "other.com",
			expected: false,
		},
		{
			name: "Exclude mode - host in list",
			config: &Config{
				FilterHostMode: "exclude",
				FilterHostList: []string{"skip.com", "ignore.com"},
			},
			host:     "skip.com",
			expected: false,
		},
		{
			name: "Exclude mode - host not in list",
			config: &Config{
				FilterHostMode: "exclude",
				FilterHostList: []string{"skip.com", "ignore.com"},
			},
			host:     "example.com",
			expected: true,
		},
		{
			name: "Request host uppercase matches normalized list",
			config: &Config{
				FilterHostMode: "include",
				FilterHostList: []string{"example.com"},
			},
			host:     "EXAMPLE.COM",
			expected: true,
		},
		{
			name: "Empty list with mode",
			config: &Config{
				FilterHostMode: "include",
				FilterHostList: []string{},
			},
			host:     "example.com",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewFilter(tt.config)
			result := filter.shouldLogHost(tt.host)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// Helper functions
func createWebSocketRequest() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	return req
}

func createRequestWithHost(host string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "http://"+host+"/test", nil)
	req.Host = host
	return req
}
