package traefik_hawkeye

import (
	"errors"
	"strings"
)

type Config struct {
	Endpoint               string   `json:"endpoint,omitempty"`
	QueueSize              int      `json:"queueSize,omitempty"`
	BatchSize              int      `json:"batchSize,omitempty"`
	FlushEveryMs           int      `json:"flushEveryMs,omitempty"`
	HTTPTimeoutMs          int      `json:"httpTimeoutMs,omitempty"`
	IncludeRequestHeaders  []string `json:"includeRequestHeaders,omitempty"`
	IncludeResponseHeaders []string `json:"includeResponseHeaders,omitempty"`
	FilterHostMode         string   `json:"filterHostMode,omitempty"`
	FilterHostList         []string `json:"filterHostList,omitempty"`
	FilterContentTypeMode  string   `json:"filterContentTypeMode,omitempty"`
	FilterContentTypeList  []string `json:"filterContentTypeList,omitempty"`
}

func CreateConfig() *Config {
	return &Config{
		QueueSize:              500,
		BatchSize:              50,
		FlushEveryMs:           3000,
		HTTPTimeoutMs:          300,
		IncludeRequestHeaders:  []string{},
		IncludeResponseHeaders: []string{},
	}
}

func validateConfig(config *Config) error {
	if config == nil {
		return errors.New("config is nil")
	}

	var issues []string

	if config.QueueSize <= 0 {
		issues = append(issues, "queueSize must be greater than 0")
	}
	if config.BatchSize <= 0 {
		issues = append(issues, "batchSize must be greater than 0")
	}
	if config.FlushEveryMs <= 0 {
		issues = append(issues, "flushEveryMs must be greater than 0")
	}
	if config.HTTPTimeoutMs <= 0 {
		issues = append(issues, "httpTimeoutMs must be greater than 0")
	}

	if config.FilterHostMode != "" && config.FilterHostMode != "include" && config.FilterHostMode != "exclude" {
		issues = append(issues, "filterHostMode must be either 'include', 'exclude', or empty")
	}
	if config.FilterContentTypeMode != "" && config.FilterContentTypeMode != "include" && config.FilterContentTypeMode != "exclude" {
		issues = append(issues, "filterContentTypeMode must be either 'include', 'exclude', or empty")
	}

	if len(issues) > 0 {
		return errors.New(strings.Join(issues, "; "))
	}

	return nil
}

func normalizeConfig(config *Config) {
	if config == nil {
		return
	}

	if config.IncludeRequestHeaders == nil {
		config.IncludeRequestHeaders = []string{}
	}
	config.IncludeRequestHeaders = normalizeHeaders(config.IncludeRequestHeaders)

	if config.IncludeResponseHeaders == nil {
		config.IncludeResponseHeaders = []string{}
	}
	config.IncludeResponseHeaders = normalizeHeaders(config.IncludeResponseHeaders)

	config.FilterHostList = normalizeStrings(config.FilterHostList)
	config.FilterContentTypeList = normalizeStrings(config.FilterContentTypeList)

	if config.FilterHostMode != "" {
		config.FilterHostMode = strings.ToLower(strings.TrimSpace(config.FilterHostMode))
	}
	if config.FilterContentTypeMode != "" {
		config.FilterContentTypeMode = strings.ToLower(strings.TrimSpace(config.FilterContentTypeMode))
	}
}

func normalizeHeaders(headers []string) []string {
	if headers == nil {
		return []string{}
	}

	out := make([]string, 0, len(headers))
	seen := make(map[string]struct{}, len(headers))

	for _, h := range headers {
		trimmed := strings.TrimSpace(h)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}

	return out
}

func normalizeStrings(values []string) []string {
	if values == nil {
		return []string{}
	}

	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		out = append(out, strings.ToLower(trimmed))
	}

	return out
}
