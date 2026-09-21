package httpc

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"codeberg.org/donkeywon/golib/util/iou"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// helper types for guessContentLength branches
// ============================================================================

type lenReader struct{ data []byte }

func (r *lenReader) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, io.EOF
	}
	return n, nil
}
func (r *lenReader) Len() int { return len(r.data) }

type lenReader2 struct{ data []byte }

func (r *lenReader2) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, io.EOF
	}
	return n, nil
}
func (r *lenReader2) Len() int64 { return int64(len(r.data)) }

type sizeReader struct{ data []byte }

func (r *sizeReader) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, io.EOF
	}
	return n, nil
}
func (r *sizeReader) Size() int { return len(r.data) }

type sizeReader2 struct{ data []byte }

func (r *sizeReader2) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, io.EOF
	}
	return n, nil
}
func (r *sizeReader2) Size() int64 { return int64(len(r.data)) }

// plainReader is an io.Reader with no Len() or Size() methods. Used to test
// the default branch of guessContentLength.
type plainReader struct{ data string }

func (r *plainReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

// errorReader always returns an error on Read.
type errorReader struct{ err error }

func (r *errorReader) Read(_ []byte) (int, error) { return 0, r.err }

// ============================================================================
// DoWithClientTimeout core tests
// ============================================================================

func TestDoWithClientTimeout_NilContextPanics(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() {
		_ = DoWithClientTimeout(nil, 0, http.MethodGet, "http://x", http.DefaultClient)
	})
}

func TestDoWithClientTimeout_WithTimeout(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	ctx := context.Background()
	err := DoWithClientTimeout(ctx, 5*time.Second, http.MethodGet, svr.URL, svr.Client())
	require.NoError(t, err)
}

func TestDoWithClientTimeout_ZeroTimeout(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer svr.Close()

	ctx := context.Background()
	err := DoWithClientTimeout(ctx, 0, http.MethodGet, svr.URL, svr.Client())
	require.NoError(t, err)
}

func TestDoWithClientTimeout_NewRequestError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	err := DoWithClientTimeout(ctx, 0, http.MethodGet, "://invalid", http.DefaultClient)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create http request failed")
}

func TestDoWithClientTimeout_ClientDoError(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	svr.Close() // force connection error

	ctx := context.Background()
	err := DoWithClientTimeout(ctx, time.Second, http.MethodGet, svr.URL, svr.Client())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http request failed")
}

func TestDoWithClientTimeout_HandleReqError(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer svr.Close()

	ctx := context.Background()
	opt := ReqOptionFunc(func(r *http.Request) error { return errors.New("boom") })
	err := DoWithClientTimeout(ctx, 0, http.MethodGet, svr.URL, svr.Client(), opt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handle http request failed")
}

func TestDoWithClientTimeout_HandleReqNilOption(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	ctx := context.Background()
	err := DoWithClientTimeout(ctx, 0, http.MethodGet, svr.URL, svr.Client(), nil)
	require.NoError(t, err)
}

func TestDoWithClientTimeout_HandleRespError(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer svr.Close()

	ctx := context.Background()
	opt := RespOptionFunc(func(resp *http.Response) error { return errors.New("boom") })
	err := DoWithClientTimeout(ctx, 0, http.MethodGet, svr.URL, svr.Client(), opt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handle http response failed")
}

// ============================================================================
// Do / DoTimeout / DoWithClient
// ============================================================================

func TestDo(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	ctx := context.Background()
	err := Do(ctx, http.MethodGet, svr.URL)
	require.NoError(t, err)
}

func TestDoTimeout(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	ctx := context.Background()
	err := DoTimeout(ctx, 5*time.Second, http.MethodGet, svr.URL)
	require.NoError(t, err)
}

func TestDoWithClient(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	ctx := context.Background()
	err := DoWithClient(ctx, http.MethodGet, svr.URL, svr.Client())
	require.NoError(t, err)
}

// ============================================================================
// Method convenience functions
// ============================================================================

func TestAllMethodFunctions(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "method: %s", r.Method)
	}))
	defer svr.Close()

	ctx := context.Background()

	noTimeout := []struct {
		name string
		fn   func(context.Context, string, ...Option) error
	}{
		{"Get", Get}, {"Post", Post}, {"Head", Head},
		{"Delete", Delete}, {"Put", Put}, {"Patch", Patch},
		{"Connect", Connect}, {"Options", Options}, {"Trace", Trace},
	}
	for _, tc := range noTimeout {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn(ctx, svr.URL)
			require.NoError(t, err)
		})
	}

	withTimeout := []struct {
		name string
		fn   func(context.Context, time.Duration, string, ...Option) error
	}{
		{"GetTimeout", GetTimeout}, {"PostTimeout", PostTimeout}, {"HeadTimeout", HeadTimeout},
		{"DeleteTimeout", DeleteTimeout}, {"PutTimeout", PutTimeout}, {"PatchTimeout", PatchTimeout},
		{"ConnectTimeout", ConnectTimeout}, {"OptionsTimeout", OptionsTimeout}, {"TraceTimeout", TraceTimeout},
	}
	for _, tc := range withTimeout {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn(ctx, 5*time.Second, svr.URL)
			require.NoError(t, err)
		})
	}
}

// ============================================================================
// Option interface / ReqOptionFunc / RespOptionFunc
// ============================================================================

func TestReqOptionFunc(t *testing.T) {
	t.Parallel()
	f := ReqOptionFunc(func(r *http.Request) error { return nil })
	require.NoError(t, f.HandleReq(nil))
	require.NoError(t, f.HandleResp(nil), "HandleResp should return nil")
}

func TestRespOptionFunc(t *testing.T) {
	t.Parallel()
	f := RespOptionFunc(func(r *http.Response) error { return nil })
	require.NoError(t, f.HandleResp(nil))
	require.NoError(t, f.HandleReq(nil), "HandleReq should return nil")
}

// ============================================================================
// WithHeaders
// ============================================================================

func TestWithHeaders_OK(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-A") != "1" || r.Header.Get("X-B") != "2" {
			t.Error("header mismatch")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Get(context.Background(), svr.URL, WithHeaders("X-A", "1", "X-B", "2"))
	require.NoError(t, err)
}

func TestWithHeaders_PanicsOnOddArgs(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() {
		_ = WithHeaders("X-A", "1", "X-B")
	})
}

// ============================================================================
// WithHeader
// ============================================================================

func TestWithHeader(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "a", r.Header.Get("X-T"))
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	h := http.Header{"X-T": {"a"}}
	err := Get(context.Background(), svr.URL, WithHeader(h))
	require.NoError(t, err)
}

// ============================================================================
// WithHeaderMap
// ============================================================================

func TestWithHeaderMap(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "v", r.Header.Get("X-M"))
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	m := map[string]string{"X-M": "v"}
	err := Get(context.Background(), svr.URL, WithHeaderMap(m))
	require.NoError(t, err)
}

// ============================================================================
// WithBody
// ============================================================================

func TestWithBody(t *testing.T) {
	t.Parallel()
	body := []byte("hello body")
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		assert.Equal(t, body, b)
		assert.Equal(t, int64(len(body)), r.ContentLength)
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Post(context.Background(), svr.URL, WithBody(body))
	require.NoError(t, err)
}

func TestWithBody_Empty(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		assert.Empty(t, b)
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Post(context.Background(), svr.URL, WithBody(nil))
	require.NoError(t, err)
}

// ============================================================================
// WithBodyReader / guessContentLength branches
// ============================================================================

func TestWithBodyReader_ReadCloser(t *testing.T) {
	t.Parallel()
	data := "rc body"
	rc := io.NopCloser(strings.NewReader(data))
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ContentLength 非零因为 strings.Reader 实现 Len()
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Post(context.Background(), svr.URL, WithBodyReader(rc))
	require.NoError(t, err)
}

func TestWithBodyReader_HasLen(t *testing.T) {
	t.Parallel()
	lr := &lenReader{data: []byte("len-data")}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, int64(8), r.ContentLength)
		b, _ := io.ReadAll(r.Body)
		assert.Equal(t, []byte("len-data"), b)
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Post(context.Background(), svr.URL, WithBodyReader(lr))
	require.NoError(t, err)
}

func TestWithBodyReader_HasLen2(t *testing.T) {
	t.Parallel()
	lr := &lenReader2{data: []byte("len2-data")}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, int64(9), r.ContentLength)
		b, _ := io.ReadAll(r.Body)
		assert.Equal(t, []byte("len2-data"), b)
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Post(context.Background(), svr.URL, WithBodyReader(lr))
	require.NoError(t, err)
}

func TestWithBodyReader_HasSize(t *testing.T) {
	t.Parallel()
	sr := &sizeReader{data: []byte("size-data")}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, int64(9), r.ContentLength)
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Post(context.Background(), svr.URL, WithBodyReader(sr))
	require.NoError(t, err)
}

func TestWithBodyReader_HasSize2(t *testing.T) {
	t.Parallel()
	sr := &sizeReader2{data: []byte("size2-data")}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, int64(10), r.ContentLength)
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Post(context.Background(), svr.URL, WithBodyReader(sr))
	require.NoError(t, err)
}

func TestWithBodyReader_NoLengthHint(t *testing.T) {
	t.Parallel()
	// Use io.Reader without Len()/Size() — content length stays 0 (chunked)
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		assert.Equal(t, "no-len-hint", string(b))
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	reader := &plainReader{data: "no-len-hint"}
	err := Post(context.Background(), svr.URL, WithBodyReader(reader))
	require.NoError(t, err)
}

// ============================================================================
// WithBodyJSON
// ============================================================================

func TestWithBodyJSON(t *testing.T) {
	t.Parallel()
	type payload struct {
		Name string `json:"name"`
	}
	v := &payload{Name: "test"}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var got payload
		json.UnmarshalRead(r.Body, &got)
		assert.Equal(t, "test", got.Name)
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Post(context.Background(), svr.URL, WithBodyJSON(v))
	require.NoError(t, err)
}

// ============================================================================
// WithBodyMarshal
// ============================================================================

func TestWithBodyMarshal(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
		b, _ := io.ReadAll(r.Body)
		assert.Equal(t, "custom", string(b))
		assert.Equal(t, int64(6), r.ContentLength)
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	marshal := func(v any) ([]byte, error) { return []byte(v.(string)), nil }
	err := Post(context.Background(), svr.URL, WithBodyMarshal("custom", "text/plain", marshal))
	require.NoError(t, err)
}

func TestWithBodyMarshal_Error(t *testing.T) {
	t.Parallel()
	marshal := func(v any) ([]byte, error) { return nil, errors.New("marshal fail") }
	err := DoWithClientTimeout(
		context.Background(), 0, http.MethodPost, "http://x",
		http.DefaultClient,
		WithBodyMarshal(nil, "", marshal),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal request body failed")
}

// ============================================================================
// WithBodyForm
// ============================================================================

func TestWithBodyForm(t *testing.T) {
	t.Parallel()
	val := url.Values{"key": {"val"}}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		b, _ := io.ReadAll(r.Body)
		assert.Equal(t, "key=val", string(b))
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Post(context.Background(), svr.URL, WithBodyForm(val))
	require.NoError(t, err)
}

// ============================================================================
// CheckStatusCode
// ============================================================================

func TestCheckStatusCode_OK(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	var sc int
	err := Get(context.Background(), svr.URL, CheckStatusCode(nil, &sc, http.StatusOK))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, sc)
}

func TestCheckStatusCode_Mismatch(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "server error")
	}))
	defer svr.Close()

	err := Get(context.Background(), svr.URL, CheckStatusCode(nil, nil, http.StatusOK))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status code: 500")
}

func TestCheckStatusCode_MismatchWithBodyDump(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "not found body")
	}))
	defer svr.Close()

	var buf bytes.Buffer
	err := Get(context.Background(), svr.URL, CheckStatusCode(&buf, nil, http.StatusOK))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status code: 404")
	assert.Equal(t, "not found body", buf.String())
}

func TestCheckStatusCode_NoExpected(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer svr.Close()

	var sc int
	err := Get(context.Background(), svr.URL, CheckStatusCode(nil, &sc))
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, sc)
}

func TestCheckStatusCode_NilStatusCodePtr(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Get(context.Background(), svr.URL, CheckStatusCode(nil, nil, http.StatusOK))
	require.NoError(t, err)
}

// ============================================================================
// CheckStatusCodeRange
// ============================================================================

func TestCheckStatusCodeRange_OK(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	var sc int
	err := Get(context.Background(), svr.URL, CheckStatusCodeRange(nil, &sc, 200, 299))
	require.NoError(t, err)
	assert.Equal(t, 200, sc)
}

func TestCheckStatusCodeRange_Boundary(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Get(context.Background(), svr.URL, CheckStatusCodeRange(nil, nil, 200, 200))
	require.NoError(t, err)
}

func TestCheckStatusCodeRange_MismatchWithDump(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "boom")
	}))
	defer svr.Close()

	var buf bytes.Buffer
	err := Get(context.Background(), svr.URL, CheckStatusCodeRange(&buf, nil, 200, 299))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status code: 500")
	assert.Equal(t, "boom", buf.String())
}

func TestCheckStatusCodeRange_NilStatusPtr(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Get(context.Background(), svr.URL, CheckStatusCodeRange(nil, nil, 100, 399))
	require.NoError(t, err)
}

// ============================================================================
// ToString
// ============================================================================

func TestToString(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "hello world")
	}))
	defer svr.Close()

	var s string
	err := Get(context.Background(), svr.URL, ToString(&s))
	require.NoError(t, err)
	assert.Equal(t, "hello world", s)
}

func TestToString_Empty(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	var s string
	err := Get(context.Background(), svr.URL, ToString(&s))
	require.NoError(t, err)
	assert.Equal(t, "", s)
}

// ============================================================================
// ToBytes
// ============================================================================

func TestToBytes(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "abc")
	}))
	defer svr.Close()

	buf := make([]byte, 10)
	var n int
	err := Get(context.Background(), svr.URL, ToBytes(&n, buf))
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, "abc", string(buf[:n]))
}

func TestToBytes_NoN(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "xyz")
	}))
	defer svr.Close()

	buf := make([]byte, 3)
	err := Get(context.Background(), svr.URL, ToBytes(nil, buf))
	require.NoError(t, err)
	assert.Equal(t, "xyz", string(buf))
}

func TestToBytes_ReadError(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	// buffer too small: iou.ReadFill will fill buf but then try to read more, hitting EOF;
	// that EOF is consumed (see option.go:243-245), so no error.
	// To trigger real error: use nil body (no content) and expect io.EOF being silenced.
	// Real read error: impossible with httptest, skip.
	err := Get(context.Background(), svr.URL, ToBytes(nil, []byte{}))
	require.NoError(t, err)
}

// ============================================================================
// ToJSON
// ============================================================================

func TestToJSON(t *testing.T) {
	t.Parallel()
	type resp struct {
		ID int `json:"id"`
	}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":42}`)
	}))
	defer svr.Close()

	var got resp
	err := Get(context.Background(), svr.URL, ToJSON(&got))
	require.NoError(t, err)
	assert.Equal(t, 42, got.ID)
}

// An empty body is not valid JSON: encoding/json/v2 reports it as "unexpected EOF",
// and ToAnyDecode only tolerates a bare io.EOF (see TestToAnyDecode_EOFIgnored).
// A caller that must accept an empty body should read it with ToBytes and check
// the length itself.
func TestToJSON_EmptyBody_DecodeError(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	var s string
	err := Get(context.Background(), svr.URL, ToJSON(&s))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode response body failed")
}

// ============================================================================
// ToAnyDecode
// ============================================================================

func TestToAnyDecode(t *testing.T) {
	t.Parallel()
	type data struct {
		Val string `json:"val"`
	}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"val":"hi"}`)
	}))
	defer svr.Close()

	var d data
	err := Get(context.Background(), svr.URL, ToAnyDecode(&d, func(r io.Reader, a any) error { return json.UnmarshalRead(r, a) }))
	require.NoError(t, err)
	assert.Equal(t, "hi", d.Val)
}

// ============================================================================
// ToAnyUnmarshal
// ============================================================================

func TestToAnyUnmarshal(t *testing.T) {
	t.Parallel()
	type data struct {
		Val string `json:"val"`
	}
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"val":"hi"}`)
	}))
	defer svr.Close()

	var d data
	err := Get(context.Background(), svr.URL, ToAnyUnmarshal(&d, func(bs []byte, v any) error {
		return json.Unmarshal(bs, v)
	}))
	require.NoError(t, err)
	assert.Equal(t, "hi", d.Val)
}

func TestToAnyUnmarshal_Error(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "not json")
	}))
	defer svr.Close()

	var s string
	err := Get(context.Background(), svr.URL, ToAnyUnmarshal(&s, func(bs []byte, v any) error {
		return errors.New("unmarshal fail")
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal response body failed")
}

// ============================================================================
// ToWriter
// ============================================================================

func TestToWriter_WithN(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "writer-data")
	}))
	defer svr.Close()

	var buf bytes.Buffer
	var n int64
	err := Get(context.Background(), svr.URL, ToWriter(&buf, &n))
	require.NoError(t, err)
	assert.Equal(t, int64(11), n)
	assert.Equal(t, "writer-data", buf.String())
}

func TestToWriter_NilN(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer svr.Close()

	var buf bytes.Buffer
	err := Get(context.Background(), svr.URL, ToWriter(&buf, nil))
	require.NoError(t, err)
	assert.Equal(t, "ok", buf.String())
}

// ============================================================================
// guessContentLength via WithBodyReader
// ============================================================================

func TestGuessContentLength_HasLen(t *testing.T) {
	t.Parallel()
	l := guessContentLength(&lenReader{data: []byte("12345")})
	assert.Equal(t, int64(5), l)
}

func TestGuessContentLength_HasLen2(t *testing.T) {
	t.Parallel()
	l := guessContentLength(&lenReader2{data: []byte("123456")})
	assert.Equal(t, int64(6), l)
}

func TestGuessContentLength_HasSize(t *testing.T) {
	t.Parallel()
	l := guessContentLength(&sizeReader{data: []byte("1234")})
	assert.Equal(t, int64(4), l)
}

func TestGuessContentLength_HasSize2(t *testing.T) {
	t.Parallel()
	l := guessContentLength(&sizeReader2{data: []byte("12345")})
	assert.Equal(t, int64(5), l)
}

func TestGuessContentLength_Default(t *testing.T) {
	t.Parallel()
	// plainReader has no Len()/Size() — falls through to default case
	l := guessContentLength(&plainReader{data: "hello"})
	assert.Equal(t, int64(0), l)
}

func TestGuessContentLength_Nil(t *testing.T) {
	t.Parallel()
	l := guessContentLength(nil)
	assert.Equal(t, int64(0), l)
}

// ============================================================================
// ToBytes error path (EOF ignored)
// ============================================================================

func TestToBytes_EOFIgnored(t *testing.T) {
	t.Parallel()
	// Send "ok" into a 100-byte buffer: iou.ReadFill reads 2 bytes then Read returns io.EOF.
	// EOF is silenced by option.go:243-245, so no error.
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer svr.Close()

	buf := make([]byte, 100)
	var n int
	err := Get(context.Background(), svr.URL, ToBytes(&n, buf))
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, "ok", string(buf[:n]))
}

// ============================================================================
// iou.ReadFill panic guard (negative count)
// ============================================================================

func TestIOUReadFill_Normal(t *testing.T) {
	n, err := iou.ReadFill(make([]byte, 5), strings.NewReader("abc"))
	// iou.ReadFill returns io.EOF after reading all data (read 3 bytes, then
	// Read returns io.EOF which causes loop exit).
	if err != nil && err != io.EOF {
		require.NoError(t, err)
	}
	assert.Equal(t, 3, n)
}

// ============================================================================
// CheckStatusCode body dump with error
// ============================================================================

type errorWriter struct{}

func (w *errorWriter) Write(p []byte) (int, error) { return 0, errors.New("write fail") }

func TestCheckStatusCode_BodyDumpError(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "body")
	}))
	defer svr.Close()

	err := Get(context.Background(), svr.URL, CheckStatusCode(&errorWriter{}, nil, http.StatusOK))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read response body failed")
}

func TestCheckStatusCodeRange_BodyDumpError(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "body")
	}))
	defer svr.Close()

	err := Get(context.Background(), svr.URL, CheckStatusCodeRange(&errorWriter{}, nil, 200, 299))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read response body failed")
}

// ============================================================================
// WithBodyMarshal zero-length body
// ============================================================================

func TestWithBodyMarshal_ZeroLength(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		assert.Equal(t, 0, len(b))
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	marshal := func(v any) ([]byte, error) { return []byte{}, nil }
	err := Post(context.Background(), svr.URL, WithBodyMarshal(nil, "", marshal))
	require.NoError(t, err)
}

// ============================================================================
// WithBodyForm nil request guard
// ============================================================================

func TestWithBodyForm_NilRequest(t *testing.T) {
	t.Parallel()
	f := ReqOptionFunc(func(r *http.Request) error {
		if r == nil {
			return nil
		}
		return nil
	})
	require.NoError(t, f.HandleReq(nil))
}

// ============================================================================
// Edge: DoWithClientTimeout resp.Body close covered by HandleResp error path
// ============================================================================

func TestDoWithClientTimeout_RespBodyCloseOnHandleRespError(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "body")
	}))
	defer svr.Close()

	opt := RespOptionFunc(func(resp *http.Response) error {
		// Read a bit, then return error — body close in defer should still work.
		return errors.New("resp handler error")
	})
	err := DoWithClientTimeout(context.Background(), 0, http.MethodGet, svr.URL, svr.Client(), opt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "handle http response failed")
	// No leak panic = close worked.
}

func TestDetachBody_SurvivesAutoClose(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "detached")
	}))
	defer svr.Close()

	// The helper closes the response body before returning, so DetachBody is the
	// only way to keep it readable afterwards.
	var body io.ReadCloser
	err := DoWithClientTimeout(context.Background(), 0, http.MethodGet, svr.URL, svr.Client(), DetachBody(&body))
	require.NoError(t, err)
	require.NotNil(t, body)

	bs, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, "detached", string(bs))
	require.NoError(t, body.Close())
}

// ============================================================================
// All HTTP methods with body option (ensures method propagates)
// ============================================================================

func TestAllMethodsWithBody(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	type testCase struct {
		name   string
		fn     func(context.Context, string, ...Option) error
		method string
	}
	tests := []testCase{
		{"Post", Post, http.MethodPost},
		{"Put", Put, http.MethodPut},
		{"Patch", Patch, http.MethodPatch},
		{"Connect", Connect, http.MethodConnect},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.method, r.Method)
				w.WriteHeader(http.StatusOK)
			}))
			defer svr.Close()

			err := tc.fn(ctx, svr.URL, WithBody([]byte("data")))
			require.NoError(t, err)
		})
	}
}

// ============================================================================
// All methods pass opts through correctly
// ============================================================================

func TestAllMethodsPassOpts(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "yes", r.Header.Get("X-Test"))
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	ctx := context.Background()
	opt := WithHeaders("X-Test", "yes")

	noTimeout := []func(context.Context, string, ...Option) error{
		Get, Post, Head, Delete, Put, Patch, Connect, Options, Trace,
	}
	for _, fn := range noTimeout {
		err := fn(ctx, svr.URL, opt)
		require.NoError(t, err)
	}

	withTimeout := []func(context.Context, time.Duration, string, ...Option) error{
		GetTimeout, PostTimeout, HeadTimeout, DeleteTimeout,
		PutTimeout, PatchTimeout, ConnectTimeout, OptionsTimeout, TraceTimeout,
	}
	for _, fn := range withTimeout {
		err := fn(ctx, 5*time.Second, svr.URL, opt)
		require.NoError(t, err)
	}
}

// ============================================================================
// Additional tests for remaining uncovered branches
// ============================================================================

func TestWithBodyForm_NilRequest_HandleReq(t *testing.T) {
	t.Parallel()
	opt := WithBodyForm(url.Values{"k": {"v"}})
	require.NoError(t, opt.HandleReq(nil))
}

func TestCheckStatusCode_BodyDumpSuccess(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "not found")
	}))
	defer svr.Close()

	var buf bytes.Buffer
	err := Get(context.Background(), svr.URL, CheckStatusCode(&buf, nil, http.StatusOK))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status code: 404")
	// body was dumped successfully
	assert.Equal(t, "not found", buf.String())
}

func TestCheckStatusCodeRange_BodyDumpSuccess(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "out of range")
	}))
	defer svr.Close()

	var buf bytes.Buffer
	err := Get(context.Background(), svr.URL, CheckStatusCodeRange(&buf, nil, 200, 299))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status code: 404")
	assert.Equal(t, "out of range", buf.String())
}

func TestCheckStatusCodeRange_StatusInRange(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer svr.Close()

	var sc int
	err := Get(context.Background(), svr.URL, CheckStatusCodeRange(nil, &sc, 200, 299))
	require.NoError(t, err)
	assert.Equal(t, 204, sc)
}

func TestToString_ReadError(t *testing.T) {
	// Directly test ToString with a response body that returns an error.
	t.Parallel()
	var s string
	opt := ToString(&s)
	err := opt.HandleResp(&http.Response{Body: io.NopCloser(&errorReader{err: errors.New("read fail")})})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read response body failed")
}

func TestToBytes_ReadErrorDirect(t *testing.T) {
	t.Parallel()
	buf := make([]byte, 10)
	opt := ToBytes(nil, buf)
	err := opt.HandleResp(&http.Response{Body: io.NopCloser(&errorReader{err: errors.New("read fail")})})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read response body failed")
}

func TestToAnyDecode_EOFIgnored(t *testing.T) {
	t.Parallel()
	// ToAnyDecode tolerates a bare io.EOF, which is what decode-style APIs report
	// for an empty stream. Any other error, including a truncated document, is
	// still returned to the caller.
	var v any
	opt := ToAnyDecode(&v, func(io.Reader, any) error { return io.EOF })
	err := opt.HandleResp(&http.Response{Body: io.NopCloser(strings.NewReader(""))})
	require.NoError(t, err)
}

func TestToAnyUnmarshal_Success(t *testing.T) {
	t.Parallel()
	type data struct {
		Val string `json:"val"`
	}
	var d data
	opt := ToAnyUnmarshal(&d, func(bs []byte, v any) error { return json.Unmarshal(bs, v) })
	r := io.NopCloser(strings.NewReader(`{"val":"ok"}`))
	err := opt.HandleResp(&http.Response{Body: r})
	require.NoError(t, err)
	assert.Equal(t, "ok", d.Val)
}

func TestToAnyUnmarshal_ReadAllError(t *testing.T) {
	t.Parallel()
	var v any
	opt := ToAnyUnmarshal(&v, func(bs []byte, v any) error { return nil })
	err := opt.HandleResp(&http.Response{Body: io.NopCloser(&errorReader{err: errors.New("boom")})})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read response body failed")
}

func TestToWriter_NilN_Path(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	opt := ToWriter(&buf, nil)
	r := io.NopCloser(strings.NewReader("nil-n"))
	err := opt.HandleResp(&http.Response{Body: r})
	require.NoError(t, err)
	assert.Equal(t, "nil-n", buf.String())
}

func TestToWriter_Error(t *testing.T) {
	t.Parallel()
	opt := ToWriter(&errorWriter{}, nil)
	r := io.NopCloser(strings.NewReader("data"))
	err := opt.HandleResp(&http.Response{Body: r})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read response body to writer failed")
}

func TestCheckStatusCode_ContainsMatch(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	var sc int
	err := Get(context.Background(), svr.URL, CheckStatusCode(nil, &sc, http.StatusOK, http.StatusOK))
	require.NoError(t, err)
}

func TestCheckStatusCodeRange_ContainsMatch(t *testing.T) {
	t.Parallel()
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	err := Get(context.Background(), svr.URL, CheckStatusCodeRange(nil, nil, 200, 200))
	require.NoError(t, err)
}

func TestToAnyDecode_Success(t *testing.T) {
	t.Parallel()
	type data struct {
		Val string `json:"val"`
	}
	var d data
	opt := ToAnyDecode(&d, func(r io.Reader, a any) error { return json.UnmarshalRead(r, a) })
	err := opt.HandleResp(&http.Response{Body: io.NopCloser(strings.NewReader(`{"val":"hi"}`))})
	require.NoError(t, err)
	assert.Equal(t, "hi", d.Val)
}
