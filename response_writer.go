package traefik_hawkeye

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

// responseWriter wraps http.ResponseWriter to capture status code and headers
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	headers    http.Header
}

func (rw *responseWriter) Header() http.Header {
	if rw.headers == nil {
		rw.headers = make(http.Header)
		copyHeaders(rw.headers, rw.ResponseWriter.Header())
	}
	// Return the underlying Header so writes go through
	return rw.ResponseWriter.Header()
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	// Capture headers before writing
	if rw.headers == nil {
		rw.headers = make(http.Header)
	}
	copyHeaders(rw.headers, rw.ResponseWriter.Header())
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	// Capture headers if not already captured
	if rw.headers == nil {
		rw.headers = make(http.Header)
		copyHeaders(rw.headers, rw.ResponseWriter.Header())
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support Hijacker")
}
