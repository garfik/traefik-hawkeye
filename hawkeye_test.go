package traefik_hawkeye

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestNew(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})

	config := CreateConfig()
	config.Endpoint = "http://example.com/analytics"

	handler, err := New(ctx, next, config, "hawkeye")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}
	if handler == nil {
		t.Fatal("handler is nil")
	}
}

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

func TestServeHTTP(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(100 * time.Millisecond)
	}()

	var receivedBatches []string
	var totalEvents int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		var events []Event
		if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
			t.Errorf("failed to decode events: %v", err)
		}

		mu.Lock()
		receivedBatches = append(receivedBatches, r.URL.Path)
		totalEvents += len(events)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("OK"))
	})

	config := CreateConfig()
	config.Endpoint = server.URL
	config.QueueSize = 10
	config.BatchSize = 2 // Small batch size for testing
	config.FlushEveryMs = 500

	handler, err := New(ctx, next, config, "hawkeye")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 3; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
		req.Header.Set("User-Agent", "test-agent")
		req.Header.Set("Referer", "http://referer.com")

		handler.ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Errorf("expected status OK, got %d", recorder.Code)
		}
		if recorder.Body.String() != "OK" {
			t.Errorf("expected body 'OK', got %s", recorder.Body.String())
		}
	}

	// Wait for batches to be sent (with retries for timing issues)
	var batchCount, eventsCount int
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		batchCount = len(receivedBatches)
		eventsCount = totalEvents
		mu.Unlock()
		// Wait until we receive all 3 events or timeout
		if eventsCount >= 3 {
			break
		}
	}

	// We sent 3 events, so we should receive all 3
	if batchCount == 0 {
		t.Fatal("no batches received - events were not sent to the endpoint")
	}
	if eventsCount != 3 {
		t.Errorf("expected 3 events total, got %d (received in %d batches)", eventsCount, batchCount)
	}
}

func TestNonBlockingQueue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(100 * time.Millisecond)
	}()

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
	})

	config := CreateConfig()
	config.Endpoint = "http://example.com/analytics"
	config.QueueSize = 2 // Small queue to test dropping
	config.BatchSize = 10
	config.FlushEveryMs = 1000

	handler, err := New(ctx, next, config, "hawkeye")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 10; i++ {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
		handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Errorf("request %d: expected status OK, got %d", i, recorder.Code)
		}
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", recorder.Code)
	}
}

func TestEventCreation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
	})

	config := CreateConfig()
	config.Endpoint = "http://example.com/analytics"
	config.IncludeRequestHeaders = []string{"X-Request-Id", "X-Service-Name"}

	handler, err := New(ctx, next, config, "hawkeye")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	h, ok := handler.(*hawkeye)
	if !ok {
		t.Fatal("handler is not *hawkeye type")
	}

	req := httptest.NewRequest(http.MethodPost, "https://example.com/api/test?foo=bar", nil)
	req.Header.Set("X-Real-Ip", "192.168.1.100")
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Header.Set("User-Agent", "test-agent/1.0")
	req.Header.Set("Referer", "http://google.com")
	req.Header.Set("X-Request-Id", "req-123")
	req.Header.Set("X-Service-Name", "test-service")
	req.Header.Set("X-Other-Header", "should-not-appear")

	event := h.createEvent(req, http.StatusNotFound, make(http.Header), 42)

	if event.IP != "192.168.1.100" {
		t.Errorf("expected IP 192.168.1.100, got %s", event.IP)
	}
	if event.Method != http.MethodPost {
		t.Errorf("expected method POST, got %s", event.Method)
	}
	if event.Scheme != "https" {
		t.Errorf("expected scheme https, got %s", event.Scheme)
	}
	if event.Host != "example.com" {
		t.Errorf("expected host example.com, got %s", event.Host)
	}
	if event.Path != "/api/test" {
		t.Errorf("expected path /api/test, got %s", event.Path)
	}
	if event.Status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", event.Status)
	}
	if event.DurMs != 42 {
		t.Errorf("expected duration 42, got %d", event.DurMs)
	}
	if event.UA != "test-agent/1.0" {
		t.Errorf("expected UA test-agent/1.0, got %s", event.UA)
	}
	if event.Ref != "http://google.com" {
		t.Errorf("expected referer http://google.com, got %s", event.Ref)
	}

	if event.RequestHdr["X-Request-Id"] != "req-123" {
		t.Errorf("expected X-Request-Id req-123, got %s", event.RequestHdr["X-Request-Id"])
	}
	if event.RequestHdr["X-Service-Name"] != "test-service" {
		t.Errorf("expected X-Service-Name test-service, got %s", event.RequestHdr["X-Service-Name"])
	}
	if event.RequestHdr["X-Other-Header"] != "" {
		t.Errorf("expected X-Other-Header to be empty, got %s", event.RequestHdr["X-Other-Header"])
	}

	if _, err := time.Parse(time.RFC3339, event.TS); err != nil {
		t.Errorf("invalid timestamp format: %v", err)
	}
}

func TestEventJSON(t *testing.T) {
	event := &Event{
		TS:         "2025-01-15T12:34:56Z",
		IP:         "192.168.1.1",
		Method:     "GET",
		Scheme:     "https",
		Host:       "example.com",
		Path:       "/test",
		Status:     200,
		DurMs:      123,
		Ref:        "http://referer.com",
		UA:         "test-agent",
		RequestHdr: map[string]string{"X-Request-Id": "123"},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	if !strings.Contains(string(data), `"ip":"192.168.1.1"`) {
		t.Errorf("JSON does not contain expected IP field")
	}
	if !strings.Contains(string(data), `"status":200`) {
		t.Errorf("JSON does not contain expected status field")
	}
	if !strings.Contains(string(data), `"request_hdr":{"X-Request-Id":"123"}`) {
		t.Errorf("JSON does not contain expected request_hdr field")
	}
}

func TestEventJSONWithEmptyHeaders(t *testing.T) {
	event := &Event{
		TS:          "2025-01-15T12:34:56Z",
		IP:          "192.168.1.1",
		Method:      "GET",
		Scheme:      "https",
		Host:        "example.com",
		Path:        "/test",
		Status:      200,
		DurMs:       123,
		Ref:         "http://referer.com",
		UA:          "test-agent",
		RequestHdr:  make(map[string]string),
		ResponseHdr: make(map[string]string),
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	if !strings.Contains(string(data), `"request_hdr":{}`) {
		t.Errorf("JSON does not contain empty request_hdr field, got: %s", string(data))
	}
	if !strings.Contains(string(data), `"response_hdr":{}`) {
		t.Errorf("JSON does not contain empty response_hdr field, got: %s", string(data))
	}
}

func TestIncludeResponseHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("X-Request-Id", "response-req-123")
		rw.Header().Set("X-Session-Id", "session-456")
		rw.Header().Set("X-Other-Header", "should-not-appear")
		rw.WriteHeader(http.StatusOK)
	})

	config := CreateConfig()
	config.Endpoint = "http://example.com/analytics"
	config.IncludeResponseHeaders = []string{"X-Request-Id", "X-Session-Id"}

	handler, err := New(ctx, next, config, "hawkeye")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	h, ok := handler.(*hawkeye)
	if !ok {
		t.Fatal("handler is not *hawkeye type")
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	responseHeaders := make(http.Header)
	responseHeaders.Set("X-Request-Id", "response-req-123")
	responseHeaders.Set("X-Session-Id", "session-456")
	responseHeaders.Set("X-Other-Header", "should-not-appear")

	event := h.createEvent(req, http.StatusOK, responseHeaders, 100)

	if event.ResponseHdr["X-Request-Id"] != "response-req-123" {
		t.Errorf("expected X-Request-Id response-req-123, got %s", event.ResponseHdr["X-Request-Id"])
	}
	if event.ResponseHdr["X-Session-Id"] != "session-456" {
		t.Errorf("expected X-Session-Id session-456, got %s", event.ResponseHdr["X-Session-Id"])
	}
	if event.ResponseHdr["X-Other-Header"] != "" {
		t.Errorf("expected X-Other-Header to be empty, got %s", event.ResponseHdr["X-Other-Header"])
	}
}

func TestIncludeRequestAndResponseHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("X-Request-Id", "response-req-123")
		rw.WriteHeader(http.StatusOK)
	})

	config := CreateConfig()
	config.Endpoint = "http://example.com/analytics"
	config.IncludeRequestHeaders = []string{"X-Request-Id"}
	config.IncludeResponseHeaders = []string{"X-Request-Id"}

	handler, err := New(ctx, next, config, "hawkeye")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	h, ok := handler.(*hawkeye)
	if !ok {
		t.Fatal("handler is not *hawkeye type")
	}

	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	req.Header.Set("X-Request-Id", "request-req-123")

	responseHeaders := make(http.Header)
	responseHeaders.Set("X-Request-Id", "response-req-123")

	event := h.createEvent(req, http.StatusOK, responseHeaders, 100)

	if event.RequestHdr["X-Request-Id"] != "request-req-123" {
		t.Errorf("expected X-Request-Id request-req-123 (from request), got %s", event.RequestHdr["X-Request-Id"])
	}
	if event.ResponseHdr["X-Request-Id"] != "response-req-123" {
		t.Errorf("expected X-Request-Id response-req-123 (from response), got %s", event.ResponseHdr["X-Request-Id"])
	}
}

func TestServeHTTPWithResponseHeaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(100 * time.Millisecond)
	}()

	var capturedEvents []Event
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var events []Event
		if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
			t.Errorf("failed to decode events: %v", err)
			return
		}

		mu.Lock()
		capturedEvents = append(capturedEvents, events...)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("X-Request-Id", "response-req-456")
		rw.Header().Set("X-Session-Id", "session-789")
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("OK"))
	})

	config := CreateConfig()
	config.Endpoint = server.URL
	config.QueueSize = 10
	config.BatchSize = 1 // Small batch size for immediate flush
	config.FlushEveryMs = 500
	config.IncludeResponseHeaders = []string{"X-Request-Id", "X-Session-Id"}

	handler, err := New(ctx, next, config, "hawkeye")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", recorder.Code)
	}

	time.Sleep(600 * time.Millisecond)

	mu.Lock()
	if len(capturedEvents) == 0 {
		mu.Unlock()
		t.Fatal("no events captured")
	}

	event := capturedEvents[0]
	mu.Unlock()

	if event.ResponseHdr["X-Request-Id"] != "response-req-456" {
		t.Errorf("expected X-Request-Id response-req-456, got %s", event.ResponseHdr["X-Request-Id"])
	}
	if event.ResponseHdr["X-Session-Id"] != "session-789" {
		t.Errorf("expected X-Session-Id session-789, got %s", event.ResponseHdr["X-Session-Id"])
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

func TestServeHTTPWithWebSocket(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(100 * time.Millisecond)
	}()

	var eventsReceived int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var events []Event
		if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
			return // Ignore decode errors
		}

		mu.Lock()
		eventsReceived += len(events)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	originalWriter := &testHijackerWriter{
		ResponseWriter: httptest.NewRecorder(),
	}

	next := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Simulate WebSocket handler - just return switching protocols status
		rw.WriteHeader(http.StatusSwitchingProtocols)
	})

	config := CreateConfig()
	config.Endpoint = server.URL
	config.QueueSize = 10
	config.BatchSize = 1
	config.FlushEveryMs = 500

	handler, err := New(ctx, next, config, "hawkeye")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Test WebSocket upgrade request - should NOT create event
	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "test-key")
	req.Header.Set("Sec-WebSocket-Version", "13")

	handler.ServeHTTP(originalWriter, req)

	// Wait a bit to ensure no events are sent
	time.Sleep(600 * time.Millisecond)

	mu.Lock()
	wsEvents := eventsReceived
	mu.Unlock()

	if wsEvents != 0 {
		t.Errorf("WebSocket request should not create events, but got %d events", wsEvents)
	}

	// Test regular HTTP request - should create event
	recorder := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "http://example.com/test", nil)

	next2 := http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write([]byte("OK"))
	})

	handler2, err := New(ctx, next2, config, "hawkeye")
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	handler2.ServeHTTP(recorder, req2)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status OK, got %d", recorder.Code)
	}

	// Wait for event to be sent
	time.Sleep(600 * time.Millisecond)

	mu.Lock()
	httpEvents := eventsReceived
	mu.Unlock()

	if httpEvents != 1 {
		t.Errorf("HTTP request should create 1 event, but got %d events", httpEvents)
	}
}

// testHijackerWriter is a test ResponseWriter that implements Hijacker
type testHijackerWriter struct {
	http.ResponseWriter
}

func (w *testHijackerWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	// Return error to simulate hijack (we don't need actual connection for test)
	return nil, nil, fmt.Errorf("test hijack")
}
