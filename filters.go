package traefik_hawkeye

import (
	"net/http"
	"net/url"
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
func (f *Filter) ShouldLog(req *http.Request, contentType string) bool {
	// If this is a tracking pixel, then always log
	if f.ExtractTrackingPixelPath(req) != "" {
		return true
	}

	return f.shouldLogContentType(contentType)
}

// ExtractTrackingPixelPath extracts the path from u parameter if this is a valid tracking pixel request
func (f *Filter) ExtractTrackingPixelPath(req *http.Request) string {
	if f.config.TrackingPixelURL != "" && req.URL.Path == f.config.TrackingPixelURL {
		if uParam := req.URL.Query().Get("u"); uParam != "" {
			decodedPath, err := url.QueryUnescape(uParam)
			if err == nil && decodedPath != "" {
				return decodedPath
			}
		}
	}
	return ""
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
