package traefik_hawkeye

import (
	"net"
	"net/http"
	"strings"
)

func extractIP(req *http.Request) string {
	if ip := req.Header.Get("X-Real-Ip"); ip != "" {
		return ip
	}

	if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return host
}

func extractScheme(req *http.Request) string {
	if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
		return strings.ToLower(proto)
	}

	if req.TLS != nil {
		return "https"
	}

	return "http"
}

func normalizeContentType(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}
	return strings.ToLower(contentType)
}

func copyHeaders(dst, src http.Header) {
	for k, v := range src {
		dst[k] = make([]string, len(v))
		copy(dst[k], v)
	}
}

func isWebSocketUpgrade(req *http.Request) bool {
	return strings.ToLower(req.Header.Get("Upgrade")) == "websocket" &&
		strings.Contains(strings.ToLower(req.Header.Get("Connection")), "upgrade")
}
