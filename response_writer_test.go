package traefik_hawkeye

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResponseWriterWriteHeaderCapturesStatusAndHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("X-Test", "before")

	wrapper := &responseWriter{ResponseWriter: rec}
	wrapper.Header().Set("X-Added", "value")
	wrapper.WriteHeader(http.StatusCreated)

	if wrapper.statusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, wrapper.statusCode)
	}
	if wrapper.headers.Get("X-Test") != "before" {
		t.Fatalf("expected copied header X-Test=before, got %s", wrapper.headers.Get("X-Test"))
	}
	if wrapper.headers.Get("X-Added") != "value" {
		t.Fatalf("expected copied header X-Added=value, got %s", wrapper.headers.Get("X-Added"))
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected recorder status %d, got %d", http.StatusCreated, rec.Code)
	}
}

func TestResponseWriterWriteCapturesDefaultStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Header().Set("X-Resp", "value")

	wrapper := &responseWriter{ResponseWriter: rec}
	if _, err := wrapper.Write([]byte("hi")); err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}

	if wrapper.statusCode != http.StatusOK {
		t.Fatalf("expected default status OK, got %d", wrapper.statusCode)
	}
	if wrapper.headers.Get("X-Resp") != "value" {
		t.Fatalf("expected copied header X-Resp=value, got %s", wrapper.headers.Get("X-Resp"))
	}
}

func TestResponseWriterHijackDelegates(t *testing.T) {
	rec := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	wrapper := &responseWriter{ResponseWriter: rec}

	conn, rw, err := wrapper.Hijack()
	if err != nil {
		t.Fatalf("unexpected hijack error: %v", err)
	}
	if conn != nil || rw != nil {
		t.Fatalf("expected nil conn/rw, got conn=%v rw=%v", conn, rw)
	}
	if !rec.hijacked {
		t.Fatalf("expected underlying hijack to be called")
	}
}

func TestResponseWriterHijackNotSupported(t *testing.T) {
	wrapper := &responseWriter{ResponseWriter: httptest.NewRecorder()}
	if _, _, err := wrapper.Hijack(); err == nil {
		t.Fatalf("expected error when Hijacker not supported")
	}
}

type hijackableRecorder struct {
	*httptest.ResponseRecorder
	hijacked bool
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, nil
}

func (h *hijackableRecorder) Write(b []byte) (int, error) {
	return h.ResponseRecorder.Write(b)
}

func (h *hijackableRecorder) WriteHeader(statusCode int) {
	h.ResponseRecorder.WriteHeader(statusCode)
}

func (h *hijackableRecorder) Header() http.Header {
	return h.ResponseRecorder.Header()
}
