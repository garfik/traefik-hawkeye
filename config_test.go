package traefik_hawkeye

import (
	"reflect"
	"strings"
	"testing"
)

func TestCreateConfig(t *testing.T) {
	config := CreateConfig()
	if config == nil {
		t.Fatal("CreateConfig returned nil")
	}
	if config.QueueSize != 500 {
		t.Errorf("expected QueueSize 500, got %d", config.QueueSize)
	}
	if config.BatchSize != 50 {
		t.Errorf("expected BatchSize 50, got %d", config.BatchSize)
	}
	if config.FlushEveryMs != 3000 {
		t.Errorf("expected FlushEveryMs 3000, got %d", config.FlushEveryMs)
	}
	if config.HTTPTimeoutMs != 300 {
		t.Errorf("expected HTTPTimeoutMs 300, got %d", config.HTTPTimeoutMs)
	}
}

func TestValidateConfig(t *testing.T) {
	valid := CreateConfig()
	valid.Endpoint = "http://example.com/analytics"

	if err := validateConfig(valid); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}

	noEndpoint := CreateConfig()
	noEndpoint.Endpoint = ""
	if err := validateConfig(noEndpoint); err != nil {
		t.Fatalf("expected config without endpoint to be valid, got error: %v", err)
	}

	tests := []struct {
		name        string
		config      *Config
		errContains string
	}{
		{
			name:        "nil config",
			config:      nil,
			errContains: "config is nil",
		},
		{
			name: "invalid queue size",
			config: func() *Config {
				cfg := CreateConfig()
				cfg.Endpoint = "http://example.com"
				cfg.QueueSize = 0
				return cfg
			}(),
			errContains: "queueSize must be greater than 0",
		},
		{
			name: "invalid host mode",
			config: func() *Config {
				cfg := CreateConfig()
				cfg.Endpoint = "http://example.com"
				cfg.FilterHostMode = "invalid"
				return cfg
			}(),
			errContains: "filterHostMode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("expected error to contain %q, got %v", tt.errContains, err)
			}
		})
	}
}

func TestNormalizeConfig(t *testing.T) {
	cfg := &Config{
		IncludeRequestHeaders:  []string{"  X-Test  ", "x-test", "", "X-Another"},
		IncludeResponseHeaders: nil,
		FilterHostList:         []string{" Example.com ", "API.EXAMPLE.COM", " "},
		FilterContentTypeList:  []string{" Application/JSON ", "text/HTML", ""},
		FilterHostMode:         "Include ",
		FilterContentTypeMode:  " EXCLUDE",
	}

	normalizeConfig(cfg)

	if len(cfg.IncludeRequestHeaders) != 2 {
		t.Fatalf("expected 2 unique request headers, got %d", len(cfg.IncludeRequestHeaders))
	}
	if cfg.IncludeRequestHeaders[0] != "X-Test" || cfg.IncludeRequestHeaders[1] != "X-Another" {
		t.Fatalf("unexpected request headers: %#v", cfg.IncludeRequestHeaders)
	}

	if len(cfg.IncludeResponseHeaders) != 0 {
		t.Fatalf("expected empty response headers slice, got %#v", cfg.IncludeResponseHeaders)
	}

	expectedHosts := []string{"example.com", "api.example.com"}
	if !reflect.DeepEqual(cfg.FilterHostList, expectedHosts) {
		t.Fatalf("expected hosts %v, got %v", expectedHosts, cfg.FilterHostList)
	}

	expectedContentTypes := []string{"application/json", "text/html"}
	if !reflect.DeepEqual(cfg.FilterContentTypeList, expectedContentTypes) {
		t.Fatalf("expected content types %v, got %v", expectedContentTypes, cfg.FilterContentTypeList)
	}

	if cfg.FilterHostMode != "include" {
		t.Fatalf("expected FilterHostMode to be 'include', got %q", cfg.FilterHostMode)
	}
	if cfg.FilterContentTypeMode != "exclude" {
		t.Fatalf("expected FilterContentTypeMode to be 'exclude', got %q", cfg.FilterContentTypeMode)
	}
}
