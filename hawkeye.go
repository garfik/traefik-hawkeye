package traefik_hawkeye

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
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

type Event struct {
	TS          string            `json:"ts"`
	IP          string            `json:"ip"`
	Method      string            `json:"method"`
	Scheme      string            `json:"scheme"`
	Host        string            `json:"host"`
	Path        string            `json:"path"`
	Status      int               `json:"status"`
	DurMs       int64             `json:"dur_ms"`
	Ref         string            `json:"ref"`
	UA          string            `json:"ua"`
	ContentType string            `json:"content_type"`
	RequestHdr  map[string]string `json:"request_hdr"`
	ResponseHdr map[string]string `json:"response_hdr"`
}

type hawkeye struct {
	next      http.Handler
	config    *Config
	name      string
	eventChan chan *Event
	ctx       context.Context
	cancel    context.CancelFunc
	client    *http.Client
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

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if config.QueueSize == 0 {
		config.QueueSize = 500
	}
	if config.BatchSize == 0 {
		config.BatchSize = 50
	}
	if config.FlushEveryMs == 0 {
		config.FlushEveryMs = 3000
	}
	if config.HTTPTimeoutMs == 0 {
		config.HTTPTimeoutMs = 300
	}
	if config.IncludeRequestHeaders == nil {
		config.IncludeRequestHeaders = []string{}
	}
	if config.IncludeResponseHeaders == nil {
		config.IncludeResponseHeaders = []string{}
	}
	if config.FilterHostList == nil {
		config.FilterHostList = []string{}
	}
	for i, host := range config.FilterHostList {
		config.FilterHostList[i] = strings.ToLower(strings.TrimSpace(host))
	}
	if config.FilterContentTypeList == nil {
		config.FilterContentTypeList = []string{}
	}
	for i, contentType := range config.FilterContentTypeList {
		config.FilterContentTypeList[i] = strings.ToLower(strings.TrimSpace(contentType))
	}

	pluginCtx, cancel := context.WithCancel(ctx)

	h := &hawkeye{
		next:      next,
		config:    config,
		name:      name,
		eventChan: make(chan *Event, config.QueueSize),
		ctx:       pluginCtx,
		cancel:    cancel,
		client: &http.Client{
			Timeout: time.Duration(config.HTTPTimeoutMs) * time.Millisecond,
		},
	}

	go h.worker()

	go func() {
		<-ctx.Done()
		cancel()
	}()

	return h, nil
}

func (h *hawkeye) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if isWebSocketUpgrade(req) || !h.shouldLogHost(req.Host) {
		h.next.ServeHTTP(rw, req)
		return
	}

	startTime := time.Now()
	wrapper := &responseWriter{ResponseWriter: rw, statusCode: http.StatusOK}
	h.next.ServeHTTP(wrapper, req)

	contentType := wrapper.headers.Get("Content-Type")
	if !h.shouldLogContentType(contentType) {
		return
	}

	duration := time.Since(startTime)
	event := h.createEvent(req, wrapper.statusCode, wrapper.headers, duration.Milliseconds())

	select {
	case h.eventChan <- event:
	default:
		// Channel full, drop event (no blocking, no retries)
	}
}

func (h *hawkeye) createEvent(req *http.Request, statusCode int, responseHeaders http.Header, durMs int64) *Event {
	contentType := normalizeContentType(responseHeaders.Get("Content-Type"))

	event := &Event{
		TS:          time.Now().UTC().Format(time.RFC3339),
		IP:          extractIP(req),
		Method:      req.Method,
		Scheme:      extractScheme(req),
		Host:        req.Host,
		Path:        req.URL.Path,
		Status:      statusCode,
		DurMs:       durMs,
		Ref:         req.Referer(),
		UA:          req.UserAgent(),
		ContentType: contentType,
		RequestHdr:  make(map[string]string),
		ResponseHdr: make(map[string]string),
	}

	if len(h.config.IncludeRequestHeaders) > 0 {
		for _, headerName := range h.config.IncludeRequestHeaders {
			if value := req.Header.Get(headerName); value != "" {
				event.RequestHdr[headerName] = value
			}
		}
	}

	if len(h.config.IncludeResponseHeaders) > 0 {
		for _, headerName := range h.config.IncludeResponseHeaders {
			if value := responseHeaders.Get(headerName); value != "" {
				event.ResponseHdr[headerName] = value
			}
		}
	}

	return event
}

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
	// Remove charset and other parameters, keep only the main type
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

func (h *hawkeye) worker() {
	batch := make([]*Event, 0, h.config.BatchSize)
	ticker := time.NewTicker(time.Duration(h.config.FlushEveryMs) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case event := <-h.eventChan:
			batch = append(batch, event)
			if len(batch) >= h.config.BatchSize {
				h.flushBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				h.flushBatch(batch)
				batch = batch[:0]
			}

		case <-h.ctx.Done():
			if len(batch) > 0 {
				h.flushBatch(batch)
			}
			return
		}
	}
}

func (h *hawkeye) flushBatch(batch []*Event) {
	if len(batch) == 0 || h.config.Endpoint == "" {
		return
	}

	batchCopy := make([]*Event, len(batch))
	copy(batchCopy, batch)

	go func(events []*Event) {
		payload, err := json.Marshal(events)
		if err != nil {
			return
		}

		req, err := http.NewRequestWithContext(context.Background(), "POST", h.config.Endpoint, bytes.NewReader(payload))
		if err != nil {
			return
		}

		req.Header.Set("Content-Type", "application/json")
		// Send request (ignore errors silently)
		_, _ = h.client.Do(req)
	}(batchCopy)
}

func isWebSocketUpgrade(req *http.Request) bool {
	return strings.ToLower(req.Header.Get("Upgrade")) == "websocket" &&
		strings.Contains(strings.ToLower(req.Header.Get("Connection")), "upgrade")
}

func (h *hawkeye) shouldLogHost(host string) bool {
	if len(h.config.FilterHostList) == 0 || h.config.FilterHostMode == "" {
		return true
	}

	hostLower := strings.ToLower(host)
	for _, filterHost := range h.config.FilterHostList {
		if filterHost == hostLower {
			return h.config.FilterHostMode == "include"
		}
	}

	return h.config.FilterHostMode == "exclude"
}

func (h *hawkeye) shouldLogContentType(contentType string) bool {
	if len(h.config.FilterContentTypeList) == 0 || h.config.FilterContentTypeMode == "" {
		return true
	}

	contentTypeNormalized := normalizeContentType(contentType)

	for _, filterContentType := range h.config.FilterContentTypeList {
		if filterContentType == contentTypeNormalized {
			return h.config.FilterContentTypeMode == "include"
		}
	}

	return h.config.FilterContentTypeMode == "exclude"
}

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
