package traefik_hawkeye

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type hawkeye struct {
	next      http.Handler
	config    *Config
	name      string
	eventChan chan *Event
	ctx       context.Context
	cancel    context.CancelFunc
	client    *http.Client
	filter    *Filter
}

func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	normalizeConfig(config)
	if err := validateConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "hawkeye: config validation failed: %v\n", err)
		return nil, err
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
		filter: NewFilter(config),
	}

	go h.worker()

	go func() {
		<-ctx.Done()
		cancel()
	}()

	return h, nil
}

func (h *hawkeye) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !h.filter.ShouldWrap(req) {
		h.next.ServeHTTP(rw, req)
		return
	}

	startTime := time.Now()
	wrapper := &responseWriter{ResponseWriter: rw, statusCode: http.StatusOK}
	h.next.ServeHTTP(wrapper, req)

	contentType := wrapper.headers.Get("Content-Type")
	if !h.filter.ShouldLog(req, contentType) {
		return
	}

	trackingPixelPath := h.filter.ExtractTrackingPixelPath(req)
	duration := time.Since(startTime)
	event := h.createEvent(req, wrapper.statusCode, wrapper.headers, duration.Milliseconds(), trackingPixelPath)

	select {
	case h.eventChan <- event:
	default:
		// Channel full, drop event (no blocking, no retries)
	}
}

func (h *hawkeye) createEvent(req *http.Request, statusCode int, responseHeaders http.Header, durMs int64, trackingPixelPath string) *Event {
	contentType := normalizeContentType(responseHeaders.Get("Content-Type"))

	// Use tracking pixel path if provided, otherwise use actual request path
	eventPath := req.URL.Path
	if trackingPixelPath != "" {
		eventPath = trackingPixelPath
		contentType = "text/html"
	}

	event := &Event{
		TS:          time.Now().UTC().Format(time.RFC3339),
		IP:          extractIP(req),
		Method:      req.Method,
		Scheme:      extractScheme(req),
		Host:        req.Host,
		Path:        eventPath,
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
