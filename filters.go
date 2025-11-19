package traefik_hawkeye

import (
	"net/http"
	"strings"
)

// Filter handles all filtering logic for requests
type Filter struct {
	config *Config
}

func NewFilter(config *Config) *Filter {
	return &Filter{
		config: config,
	}
}

// ShouldWrap answers if the request should be wrapped (to capture response data)
func (f *Filter) ShouldWrap(req *http.Request) bool {
	if isWebSocketUpgrade(req) {
		return false
	}

	if !f.shouldLogHost(req.Host) {
		return false
	}

	return true
}

// ShouldLog answers if the event should be logged after we got the response
func (f *Filter) ShouldLog(contentType string) bool {
	return f.shouldLogContentType(contentType)
}

func (f *Filter) shouldLogHost(host string) bool {
	if len(f.config.FilterHostList) == 0 || f.config.FilterHostMode == "" {
		return true
	}

	hostLower := strings.ToLower(host)
	for _, filterHost := range f.config.FilterHostList {
		if filterHost == hostLower {
			return f.config.FilterHostMode == "include"
		}
	}

	return f.config.FilterHostMode == "exclude"
}

func (f *Filter) shouldLogContentType(contentType string) bool {
	if len(f.config.FilterContentTypeList) == 0 || f.config.FilterContentTypeMode == "" {
		return true
	}

	contentTypeNormalized := normalizeContentType(contentType)

	for _, filterContentType := range f.config.FilterContentTypeList {
		if filterContentType == contentTypeNormalized {
			return f.config.FilterContentTypeMode == "include"
		}
	}

	return f.config.FilterContentTypeMode == "exclude"
}
