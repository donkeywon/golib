package httpio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/donkeywon/golib/util/httpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rangableServer is a test HTTP server that supports range requests.
// It tracks request counts per-method to enable retry testing.
type rangableServer struct {
	*httptest.Server
	content       []byte
	contentLength int64
	supportRange  bool

	// request counters for retry testing
	headCount atomic.Int32
	getCount  atomic.Int32

	// fail the first N GET requests with this status
	failGets   int32
	failStatus int
}

func newRangableServer(content []byte) *rangableServer {
	s := &rangableServer{
		content:       content,
		contentLength: int64(len(content)),
		supportRange:  true,
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handler))
	return s
}

func (s *rangableServer) handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodHead:
		s.headCount.Add(1)
		w.Header().Set("Content-Length", strconv.FormatInt(s.contentLength, 10))
		if s.supportRange {
			w.Header().Set("Accept-Ranges", "bytes")
		}
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		cnt := s.getCount.Add(1)
		if s.failGets > 0 && cnt <= s.failGets {
			w.WriteHeader(s.failStatus)
			return
		}

		rangeHeader := r.Header.Get("Range")
		if s.supportRange && rangeHeader != "" {
			offset, end := parseRange(rangeHeader, s.contentLength)
			if offset < 0 || end >= s.contentLength || offset > end {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			chunk := s.content[offset : end+1]
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, end, s.contentLength))
			w.Header().Set("Content-Length", strconv.Itoa(len(chunk)))
			w.WriteHeader(http.StatusPartialContent)
			w.Write(chunk)
			return
		}

		w.Header().Set("Content-Length", strconv.FormatInt(s.contentLength, 10))
		if s.supportRange {
			w.Header().Set("Accept-Ranges", "bytes")
		}
		w.WriteHeader(http.StatusOK)
		w.Write(s.content)
	}
}

// parseRange parses "bytes=X-Y" and returns offset, end (inclusive).
func parseRange(h string, maxLen int64) (offset, end int64) {
	h = strings.TrimPrefix(h, "bytes=")
	parts := strings.SplitN(h, "-", 2)
	if len(parts) != 2 {
		return -1, -1
	}
	offset, _ = strconv.ParseInt(parts[0], 10, 64)
	end, _ = strconv.ParseInt(parts[1], 10, 64)
	if end >= maxLen {
		end = maxLen - 1
	}
	return
}

// newReader is a test helper that creates a Reader using the test server's client.
func (s *rangableServer) newReader(ctx context.Context, opts ...Option) *Reader {
	return NewReader(ctx, s.URL, append([]Option{WithClient(s.Client())}, opts...)...)
}

// ── Tests ──

func TestNewReader_NilContextPanics(t *testing.T) {
	//nolint:nilcontext // intentional: test verifies panic on nil context
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic with nil context")
		}
	}()
	NewReader(nil, "http://example.com")
}

func TestRead_Range(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJ")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	buf := make([]byte, 10)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		require.NoError(t, err, "Read")
	}
	if n != 10 {
		t.Fatalf("Read got %d bytes, want 10", n)
	}
	if !bytes.Equal(buf[:n], content[:10]) {
		assert.Equal(t, content[:10], buf[:n], "content mismatch")
	}
}

func TestRead_FullContent(t *testing.T) {
	content := []byte("hello world, this is a test content")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	got, err := io.ReadAll(r)
	if err != nil {
		require.NoError(t, err, "ReadAll")
	}
	if !bytes.Equal(got, content) {
		assert.Equal(t, content, got, "content mismatch")
	}
}

func TestRead_NoRange_Fallback(t *testing.T) {
	content := []byte("no-range-content-test")
	s := newRangableServer(content)
	s.supportRange = false
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	got, err := io.ReadAll(r)
	if err != nil {
		require.NoError(t, err, "ReadAll (no-range)")
	}
	if !bytes.Equal(got, content) {
		assert.Equal(t, content, got, "content mismatch")
	}
}

func TestRead_WithOffset(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJ")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx, Offset(5))

	buf := make([]byte, 5)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		require.NoError(t, err, "ReadFull")
	}
	if !bytes.Equal(buf, content[5:10]) {
		t.Fatalf("offset content mismatch: got %q, want %q", buf, content[5:10])
	}
	// Offset should be 10 now
	if got := r.Offset(); got != 10 {
		t.Fatalf("Offset after read: got %d, want 10", got)
	}
}

func TestRead_WithLimit(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJ")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx, Limit(5))

	got, err := io.ReadAll(r)
	if err != nil {
		require.NoError(t, err, "ReadAll")
	}
	if !bytes.Equal(got, content[:5]) {
		t.Fatalf("limit content mismatch: got %q, want %q", got, content[:5])
	}
	// After reading all limited content, next read should return 0, io.EOF
	n, err := r.Read(make([]byte, 10))
	if n != 0 || err != io.EOF {
		t.Fatalf("Read after limit: got n=%d err=%v, want 0, io.EOF", n, err)
	}
}

func TestRead_WithOffsetAndLimit(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJ")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx, Offset(3), Limit(7))

	got, err := io.ReadAll(r)
	if err != nil {
		require.NoError(t, err, "ReadAll")
	}
	if !bytes.Equal(got, content[3:10]) {
		t.Fatalf("offset+limit mismatch: got %q, want %q", got, content[3:10])
	}
}

func TestReadAt(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJ")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	buf := make([]byte, 5)
	n, err := r.ReadAt(buf, 10)
	if err != nil {
		t.Fatalf("ReadAt: %v", err)
	}
	if n != 5 {
		t.Fatalf("ReadAt got %d bytes, want 5", n)
	}
	if !bytes.Equal(buf, content[10:15]) {
		t.Fatalf("ReadAt content mismatch: got %q, want %q", buf, content[10:15])
	}
}

func TestReadAt_NoRange_ReturnsError(t *testing.T) {
	content := []byte("0123456789")
	s := newRangableServer(content)
	s.supportRange = false
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	// Need to trigger init() first
	r.init()

	// init() called explicitly instead of letting ReadAt trigger it,
	// so ReadAt skips init() and goes straight to the ErrRangeUnsupported branch.
	buf := make([]byte, 5)
	_, err := r.ReadAt(buf, 2)
	if err != ErrRangeUnsupported {
		t.Fatalf("ReadAt without range: got %v, want ErrRangeUnsupported", err)
	}
}

func TestWriteTo(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJ")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	var buf bytes.Buffer
	n, err := r.WriteTo(&buf)
	if err != nil {
		require.NoError(t, err, "WriteTo")
	}
	if n != int64(len(content)) {
		assert.Equal(t, int64(len(content)), n, "WriteTo byte count")
	}
	if !bytes.Equal(buf.Bytes(), content) {
		t.Error("WriteTo content mismatch")
	}
}

func TestWriteTo_NoRange(t *testing.T) {
	content := []byte("no-range-content-test")
	s := newRangableServer(content)
	s.supportRange = false
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	var buf bytes.Buffer
	n, err := r.WriteTo(&buf)
	if err != nil {
		require.NoError(t, err, "WriteTo")
	}
	if n != int64(len(content)) {
		assert.Equal(t, int64(len(content)), n, "WriteTo byte count")
	}
	if !bytes.Equal(buf.Bytes(), content) {
		t.Error("WriteTo content mismatch")
	}
}

func TestContextCancellation_Read(t *testing.T) {
	// Use a server that never responds (context cancels before request completes)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// Context cancelled before any I/O; httpc.DoWithClient checks ctx.Err()
	// before dialing, so the transport and address never matter.
	r := NewReader(ctx, "http://127.0.0.1:1", WithClient(&http.Client{Timeout: 1 * time.Second}))

	buf := make([]byte, 10)
	_, err := r.Read(buf)
	if err == nil {
		require.Error(t, err, "expected error from cancelled context")
	}
}

func TestContextCancellation_ReadAt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := NewReader(ctx, "http://127.0.0.1:1", WithClient(&http.Client{Timeout: 1 * time.Second}))

	buf := make([]byte, 10)
	_, err := r.ReadAt(buf, 0)
	if err == nil {
		require.Error(t, err, "expected error from cancelled context")
	}
}

func TestContextCancellation_WriteTo(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := NewReader(ctx, "http://127.0.0.1:1", WithClient(&http.Client{Timeout: 1 * time.Second}))

	var buf bytes.Buffer
	_, err := r.WriteTo(&buf)
	if err == nil {
		require.Error(t, err, "expected error from cancelled context")
	}
}

func TestClose(t *testing.T) {
	content := []byte("0123456789")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	err := r.Close()
	if err != nil {
		require.NoError(t, err, "Close")
	}

	// Double close should be safe (idempotent)
	err = r.Close()
	if err != nil {
		require.NoError(t, err, "second Close")
	}

	// Read after close should return context error
	_, err = r.Read(make([]byte, 10))
	if err == nil {
		require.Error(t, err, "Read after Close should fail")
	}
}

func TestLen(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJ")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	// Before init, Len returns -1
	if got := r.Len(); got != -1 {
		t.Fatalf("Len before init: got %d, want -1", got)
	}

	// Trigger init
	r.init()

	// After init, Len should be contentLength
	if got := r.Len(); got != int64(len(content)) {
		t.Fatalf("Len after init: got %d, want %d", got, len(content))
	}
}

func TestSize(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJ")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	// Before init
	if got := r.Size(); got != -1 {
		t.Fatalf("Size before init: got %d, want -1", got)
	}

	r.init()

	if got := r.Size(); got != int64(len(content)) {
		t.Fatalf("Size after init: got %d, want %d", got, len(content))
	}
}

func TestSize_WithOffset(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJ")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx, Offset(5))

	r.init()

	// Size should be from opt.offset to end
	if got := r.Size(); got != int64(len(content))-5 {
		t.Fatalf("Size with offset 5: got %d, want %d", got, int64(len(content))-5)
	}
}

func TestRead_EmptySlice(t *testing.T) {
	content := []byte("test")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	n, err := r.Read(nil)
	if err != nil {
		t.Fatalf("Read(nil): unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("Read(nil): got %d bytes, want 0", n)
	}
}

func TestReadAt_EmptySlice(t *testing.T) {
	content := []byte("test")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	n, err := r.ReadAt(nil, 0)
	if err != nil {
		t.Fatalf("ReadAt(nil): unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("ReadAt(nil): got %d bytes, want 0", n)
	}
}

func TestRetry_GetFailsOnce(t *testing.T) {
	content := []byte("retry-test-content-0123456789")
	s := newRangableServer(content)
	s.failGets = 1 // fail first GET
	s.failStatus = http.StatusInternalServerError
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx, Retry(3))

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll with retry: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("retry content mismatch: got %q, want %q", got, content)
	}
	// First GET fails (failGets=1), retry succeeds. Total GETs: 2.
	if cnt := s.getCount.Load(); cnt != 2 {
		t.Fatalf("expected exactly 2 GET requests, got %d", cnt)
	}
}

func TestRetry_GetFailsTwice(t *testing.T) {
	content := []byte("multi-retry-content")
	s := newRangableServer(content)
	s.failGets = 2 // fail first two GETs
	s.failStatus = http.StatusServiceUnavailable
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx, Retry(4))

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll with 2 failures: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("multi-retry content mismatch")
	}
	// Two GETs fail, third succeeds. Total: 3.
	if cnt := s.getCount.Load(); cnt != 3 {
		t.Fatalf("expected exactly 3 GET requests, got %d", cnt)
	}
}

func TestRetry_Exhausted(t *testing.T) {
	content := []byte("will-fail-all")
	s := newRangableServer(content)
	s.failGets = 10 // fail more than retry attempts
	s.failStatus = http.StatusInternalServerError
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx, Retry(2)) // 2 attempts total

	_, err := io.ReadAll(r)
	if err == nil {
		require.Error(t, err, "expected error when retries exhausted")
	}
}

func TestWithHTTPOptions(t *testing.T) {
	content := []byte("custom-header-test-data")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()

	// Add a custom header as httpOption — should not break the request
	customOpt := httpc.ReqOptionFunc(func(r *http.Request) error {
		r.Header.Set("X-Custom", "test-value")
		return nil
	})

	r := s.newReader(ctx, WithHTTPOptions(customOpt))

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll with custom option: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("custom option content mismatch")
	}
}

func TestWithResponseHeaderTimeout(t *testing.T) {
	content := []byte("timeout-test")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()

	// Set a generous timeout — verify it doesn't cause issues
	r := NewReader(ctx, s.URL,
		WithClient(s.Client()),
		WithResponseHeaderTimeout(30*time.Second),
	)

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll with timeout option: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("timeout option content mismatch")
	}
}

func TestOption_RetryZeroIgnored(t *testing.T) {
	content := []byte("test")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	// Retry(0) should be ignored (default retry=1 is used)
	r := s.newReader(ctx, Retry(0))

	got, err := io.ReadAll(r)
	if err != nil {
		require.NoError(t, err, "ReadAll")
	}
	if !bytes.Equal(got, content) {
		assert.Equal(t, content, got)
	}
}

func TestOption_WithClientNilIgnored(t *testing.T) {
	content := []byte("test")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	// WithClient(nil) — should be ignored, Reader creates default client.
	// Use an invalid URL? No, we pass the real URL but WithClient(nil)
	// means default client is used. With httptest, the default client
	// won't work because it needs the test server's transport.
	//
	// This test just verifies the option doesn't crash.
	r := NewReader(ctx, s.URL, WithClient(s.Client()), WithClient(nil))

	got, err := io.ReadAll(r)
	if err != nil {
		require.NoError(t, err, "ReadAll")
	}
	if !bytes.Equal(got, content) {
		assert.Equal(t, content, got)
	}
}

func TestWriteTo_WithOffset(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJ")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx, Offset(5))

	var buf bytes.Buffer
	n, err := r.WriteTo(&buf)
	if err != nil {
		require.NoError(t, err, "WriteTo")
	}
	if n != int64(len(content))-5 {
		t.Fatalf("WriteTo wrote %d bytes, want %d", n, int64(len(content))-5)
	}
	if !bytes.Equal(buf.Bytes(), content[5:]) {
		t.Fatalf("WriteTo with offset content mismatch")
	}
}

func TestRead_LargeContent(t *testing.T) {
	// Enough data to require multiple range requests
	content := make([]byte, 1<<20) // 1MB
	for i := range content {
		content[i] = byte(i % 256)
	}

	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll large: %v", err)
	}
	if !bytes.Equal(got, content) {
		assert.Equal(t, content, got)
	}
}

func TestConcurrentReadAt(t *testing.T) {
	content := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	// Run multiple concurrent ReadAt calls
	errCh := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func(offset int64) {
			buf := make([]byte, 4)
			n, err := r.ReadAt(buf, offset)
			if err != nil {
				errCh <- fmt.Errorf("ReadAt(offset=%d): %w", offset, err)
				return
			}
			expected := content[offset : offset+4]
			if !bytes.Equal(buf[:n], expected) {
				errCh <- fmt.Errorf("ReadAt(offset=%d): got %q, want %q", offset, buf[:n], expected)
				return
			}
			errCh <- nil
		}(int64(i * 8))
	}

	for i := 0; i < 4; i++ {
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
	}
}

// Ensure Reader implements standard Go interfaces
var (
	_ io.Reader   = (*Reader)(nil)
	_ io.ReaderAt = (*Reader)(nil)
	_ io.WriterTo = (*Reader)(nil)
	_ io.Closer   = (*Reader)(nil)
)

// Test the retryHead retry mechanism by creating a server that fails HEAD once.
func TestRetry_HeadRetry(t *testing.T) {
	content := []byte("head-retry-test-data")
	s := newRangableServer(content)
	s.failGets = 0 // fallback — but we need HEAD to fail
	defer s.Close()

	// For head retry to trigger, we need the HEAD response to be non-200.
	// Let's approach differently: create a per-request fail for HEAD.
	// The easiest way: make a custom server that tracks HEAD attempts.
	headFails := &atomic.Int32{}
	headFails.Store(1) // fail first HEAD

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			if cnt := headFails.Add(-1); cnt >= 0 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			rng := r.Header.Get("Range")
			if rng != "" {
				off, end := parseRange(rng, int64(len(content)))
				if off >= 0 && end < int64(len(content)) && off <= end {
					chunk := content[off : end+1]
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", off, end, len(content)))
					w.Header().Set("Content-Length", strconv.Itoa(len(chunk)))
					w.WriteHeader(http.StatusPartialContent)
					w.Write(chunk)
					return
				}
			}
			w.Header().Set("Content-Length", strconv.Itoa(len(content)))
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusOK)
			w.Write(content)
		}
	}))
	defer ts.Close()

	ctx := context.Background()
	r := NewReader(ctx, ts.URL,
		WithClient(ts.Client()),
		Retry(3), // retry up to 3 times, first HEAD fails
	)

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll after HEAD retry: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("HEAD retry content mismatch")
	}
}

// ── Additional coverage tests ──

// TestNewReader_DefaultClient covers the else branch in NewReader where
// no custom client is provided, exercising defaultTransportDialContext.
func TestNewReader_DefaultClient(t *testing.T) {
	content := []byte("default-client-transport-test")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	// No WithClient: NewReader creates its own http.Client with default transport.
	// httptest.Server listens on a real TCP port so default transport can reach it.
	r := NewReader(ctx, s.URL)

	got, err := io.ReadAll(r)
	if err != nil {
		require.NoError(t, err, "ReadAll")
	}
	if !bytes.Equal(got, content) {
		assert.Equal(t, content, got)
	}
}

// TestNewReader_ResponseHeaderTimeout_NoClient covers the responseHeaderTimeout > 0
// branch inside the default-transport creation block.
func TestNewReader_ResponseHeaderTimeout_NoClient(t *testing.T) {
	content := []byte("rht-default-client-test")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := NewReader(ctx, s.URL, WithResponseHeaderTimeout(10*time.Second))

	got, err := io.ReadAll(r)
	if err != nil {
		require.NoError(t, err, "ReadAll")
	}
	if !bytes.Equal(got, content) {
		assert.Equal(t, content, got)
	}
}

// TestClose_BeforeRead covers the Close path where respBody is nil.
func TestClose_BeforeRead(t *testing.T) {
	content := []byte("close-before-read")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	// Close without any Read — respBody is nil, mu.Lock path hits else.
	err := r.Close()
	if err != nil {
		require.NoError(t, err, "Close before read")
	}
}

// errorReader is an io.Reader that returns a given error.
type errorReader struct {
	err error
}

func (e *errorReader) Read([]byte) (int, error) {
	return 0, e.err
}

// TestReadFromRemain_BodyReadError covers the iou.ReadFill non-EOF error path
// in readFromRemain where respBody is closed and set to nil.
// Uses white-box access (r.mu, r.respBody, r.readFromRemain) to inject
// a failing reader into the internal state without a real HTTP call.
func TestReadFromRemain_BodyReadError(t *testing.T) {
	content := []byte("body-read-error-test")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	err := r.init()
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// Replace respBody with one that errors on read.
	r.mu.Lock()
	r.respBody = &respBodyReader{
		ReadCloser: io.NopCloser(&errorReader{err: errors.New("injected body read error")}),
		r:          r,
	}
	r.mu.Unlock()

	buf := make([]byte, 10)
	_, err = r.readFromRemain(buf)
	if err == nil {
		require.Error(t, err, "expected non-EOF error from readFromRemain")
	}
}

// TestRead_InitError covers Read's init() error path.
func TestRead_InitError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer s.Close()

	ctx := context.Background()
	r := NewReader(ctx, s.URL, WithClient(s.Client()))

	_, err := r.Read(make([]byte, 10))
	if err == nil {
		require.Error(t, err, "expected error from Read with failing HEAD")
	}
}

// TestReadAt_InitError covers ReadAt's init() error path.
func TestReadAt_InitError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer s.Close()

	ctx := context.Background()
	r := NewReader(ctx, s.URL, WithClient(s.Client()))

	_, err := r.ReadAt(make([]byte, 10), 0)
	if err == nil {
		require.Error(t, err, "expected error from ReadAt with failing HEAD")
	}
}

// TestReadAt_GetPartError covers ReadAt's getPart error path.
func TestReadAt_GetPartError(t *testing.T) {
	content := []byte("readat-getpart-error")
	s := newRangableServer(content)
	s.failGets = 1
	s.failStatus = http.StatusInternalServerError
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	// ReadAt does NOT retry, so a single getPart failure propagates.
	// init() called explicitly to succeed first, so ReadAt skips init()
	// and reaches the getPart() error path instead of the init error path.
	err := r.init()
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	_, err = r.ReadAt(make([]byte, 5), 0)
	if err == nil {
		require.Error(t, err, "expected error from ReadAt with failing GET")
	}
}

// TestWriteTo_InitError covers WriteTo's init() error path.
func TestWriteTo_InitError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer s.Close()

	ctx := context.Background()
	r := NewReader(ctx, s.URL, WithClient(s.Client()))

	var buf bytes.Buffer
	_, err := r.WriteTo(&buf)
	if err == nil {
		require.Error(t, err, "expected error from WriteTo with failing HEAD")
	}
}

// TestWriteTo_NoRangeError covers WriteTo's no-range get() error path.
func TestWriteTo_NoRangeError(t *testing.T) {
	content := []byte("writeto-norange-error")
	s := newRangableServer(content)
	s.supportRange = false
	s.failGets = 1
	s.failStatus = http.StatusInternalServerError
	defer s.Close()

	ctx := context.Background()
	// Retry=1 means no retry, so the failing GET propagates immediately.
	r := s.newReader(ctx, Retry(1))

	var buf bytes.Buffer
	_, err := r.WriteTo(&buf)
	if err == nil {
		require.Error(t, err, "expected error from WriteTo no-range with failing GET")
	}
}

// failingRespOpt returns an httpc.Option whose HandleResp always fails.
// This is used to trigger cleanup paths after the body has been captured.
func failingRespOpt() httpc.Option {
	return httpc.RespOptionFunc(func(*http.Response) error {
		return errors.New("injected handle-resp failure")
	})
}

// TestRemainWriteTo_GetRemainError covers the remainWriteTo error path
// where getRemain returns a non-EOF error (triggered by failing httpOptions).
func TestRemainWriteTo_GetRemainError(t *testing.T) {
	content := []byte("remain-writeto-error")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	err := r.init()
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// Inject a failing option that will cause getPart to fail after
	// RespOptionFunc runs (so getRemain's respBody cleanup path is hit).
	r.opt.httpOptions = append(r.opt.httpOptions, failingRespOpt())

	var buf bytes.Buffer
	_, err = r.remainWriteTo(&buf)
	if err == nil {
		require.Error(t, err, "expected error from remainWriteTo")
	}
}

// TestGetRemain_RespBodyCleanup covers getRemain's error path where
// respBody was set by RespOptionFunc but a later option fails, requiring cleanup.
func TestGetRemain_RespBodyCleanup(t *testing.T) {
	content := []byte("getremain-cleanup-test")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	err := r.init()
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// Inject failing option after init so it doesn't affect head().
	r.opt.httpOptions = append(r.opt.httpOptions, failingRespOpt())

	_, err = r.getRemain()
	if err == nil {
		require.Error(t, err, "expected error from getRemain")
	}
}

// TestRetryGetNoRange_OnRetryAndError covers retryGetNoRange's OnRetry
// callback (where respBody != nil) and the final error cleanup path.
func TestRetryGetNoRange_OnRetryAndError(t *testing.T) {
	content := []byte("retry-no-range-cleanup")
	s := newRangableServer(content)
	s.supportRange = false
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx, Retry(3))

	err := r.init()
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// Inject failing option: every attempt will set respBody (via RespOptionFunc)
	// then fail the HandleResp chain, triggering OnRetry + final cleanup.
	r.opt.httpOptions = append(r.opt.httpOptions, failingRespOpt())

	_, err = r.retryGetNoRange()
	if err == nil {
		require.Error(t, err, "expected error from retryGetNoRange")
	}
}

// TestRead_RetryGetNoRangeError covers Read's retryGetNoRange error path
// in the non-range fallback branch.
func TestRead_RetryGetNoRangeError(t *testing.T) {
	content := []byte("read-no-range-error")
	s := newRangableServer(content)
	s.supportRange = false
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx, Retry(1)) // no retry

	err := r.init()
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	// Inject failing option into httpOptions.
	r.opt.httpOptions = append(r.opt.httpOptions, failingRespOpt())

	_, err = r.Read(make([]byte, 10))
	if err == nil {
		require.Error(t, err, "expected error from Read with failing retryGetNoRange")
	}
}

// TestClose_AfterRead covers the Close respBody.Close() path (respBody != nil).
func TestClose_AfterRead(t *testing.T) {
	content := []byte("close-after-read-test")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	// Read some data first to populate r.respBody
	buf := make([]byte, 5)
	_, err := r.Read(buf)
	if err != nil {
		require.NoError(t, err, "Read")
	}

	// Now Close should find respBody non-nil and close it.
	err = r.Close()
	if err != nil {
		require.NoError(t, err, "Close after Read")
	}
}

// TestGetRemain_LenZero covers the getRemain n <= 0 early return (io.EOF).
func TestGetRemain_LenZero(t *testing.T) {
	content := []byte("getremain-len-zero")
	s := newRangableServer(content)
	defer s.Close()

	ctx := context.Background()
	r := s.newReader(ctx)

	// Consume all content so r.Len() becomes 0.
	got, err := io.ReadAll(r)
	if err != nil {
		require.NoError(t, err, "ReadAll")
	}
	if !bytes.Equal(got, content) {
		assert.Equal(t, content, got)
	}

	// Now r.Len() == 0, getRemain should return io.EOF immediately.
	_, err = r.getRemain()
	if err != io.EOF {
		t.Fatalf("getRemain after all content consumed: got %v, want io.EOF", err)
	}
}

// TestRetryGetNoRange_ErrorCleanup covers the final respBody.Close() in
// retryGetNoRange's error path when respBody != nil after retries exhaust.
func TestRetryGetNoRange_ErrorCleanup(t *testing.T) {
	content := []byte("no-range-error-cleanup-final")
	s := newRangableServer(content)
	s.supportRange = false
	defer s.Close()

	ctx := context.Background()
	// Retry(1): default value, single attempt total, no actual retry.
	// Failure goes through the final error check where respBody != nil.
	r := s.newReader(ctx, Retry(1))

	err := r.init()
	if err != nil {
		t.Fatalf("init: %v", err)
	}

	r.opt.httpOptions = append(r.opt.httpOptions, failingRespOpt())

	_, err = r.retryGetNoRange()
	if err == nil {
		require.Error(t, err, "expected error from retryGetNoRange")
	}
}
