package httpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/donkeywon/golib/util/httpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRespData struct {
	Message string `json:"message"`
}

func testHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(testRespData{Message: "ok"})
}

func testHandlerEchoBody(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	io.Copy(w, r.Body)
}

func testHandlerStatus(w http.ResponseWriter, r *http.Request, status int) {
	w.WriteHeader(status)
	w.Write([]byte("body"))
}

func TestAllHTTPMethods(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(testHandler))
	defer s.Close()

	tests := []struct {
		name string
		call func(ctx context.Context, url string, opts ...Option) (*http.Response, error)
	}{
		{"Get", Get},
		{"Post", Post},
		{"Head", Head},
		{"Delete", Delete},
		{"Put", Put},
		{"Patch", Patch},
		{"Connect", Connect},
		{"Options", Options},
		{"Trace", Trace},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.call(context.Background(), s.URL)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func TestAllHTTPMethodsTimeout(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(testHandler))
	defer s.Close()

	tests := []struct {
		name string
		call func(ctx context.Context, timeout time.Duration, url string, opts ...Option) (*http.Response, error)
	}{
		{"GetTimeout", GetTimeout},
		{"PostTimeout", PostTimeout},
		{"HeadTimeout", HeadTimeout},
		{"DeleteTimeout", DeleteTimeout},
		{"PutTimeout", PutTimeout},
		{"PatchTimeout", PatchTimeout},
		{"ConnectTimeout", ConnectTimeout},
		{"OptionsTimeout", OptionsTimeout},
		{"TraceTimeout", TraceTimeout},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.call(context.Background(), time.Second, s.URL)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
		})
	}
}

func TestDoWithClientNilContextPanics(t *testing.T) {
	assert.Panics(t, func() {
		DoWithClient(nil, http.MethodGet, "http://example.com", http.DefaultClient)
	})
}

func TestWithHeaders(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "value1", r.Header.Get("X-Test-1"))
		assert.Equal(t, "value2", r.Header.Get("X-Test-2"))
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	resp, err := Get(context.Background(), s.URL,
		WithHeaders("X-Test-1", "value1", "X-Test-2", "value2"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWithHeader(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "val1", r.Header.Get("X-H-1"))
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	h := http.Header{}
	h.Set("X-H-1", "val1")
	resp, err := Get(context.Background(), s.URL, WithHeader(h))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWithHeaderMap(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "v1", r.Header.Get("K1"))
		assert.Equal(t, "v2", r.Header.Get("K2"))
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	resp, err := Get(context.Background(), s.URL,
		WithHeaderMap(map[string]string{"K1": "v1", "K2": "v2"}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWithBody(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, "test body content", string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	resp, err := Post(context.Background(), s.URL, WithBody([]byte("test body content")))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWithBodyReader(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, "reader body", string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	resp, err := Post(context.Background(), s.URL,
		WithBodyReader(strings.NewReader("reader body")))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWithBodyJSON(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, httpu.MIMEJSON, r.Header.Get("Content-Type"))
		var v testReqBody
		json.NewDecoder(r.Body).Decode(&v)
		assert.Equal(t, "xyz", v.FieldA)
		assert.Equal(t, 999, v.FieldB)
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	resp, err := Post(context.Background(), s.URL,
		WithBodyJSON(&testReqBody{FieldA: "xyz", FieldB: 999}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWithBodyForm(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, httpu.MIMEPOSTForm, r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		assert.Contains(t, string(body), "key1=val1")
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	form := url.Values{}
	form.Set("key1", "val1")
	resp, err := Post(context.Background(), s.URL, WithBodyForm(form))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestCheckStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("accepted"))
	}))
	defer srv.Close()

	t.Run("matches expected", func(t *testing.T) {
		var sc int
		resp, err := Get(context.Background(), srv.URL,
			CheckStatusCode(nil, &sc, http.StatusAccepted))
		require.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, sc)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	})

	t.Run("does not match expected", func(t *testing.T) {
		_, err := Get(context.Background(), srv.URL,
			CheckStatusCode(nil, nil, http.StatusOK))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected response status code")
	})

	t.Run("no expected codes (pass-through)", func(t *testing.T) {
		resp, err := Get(context.Background(), srv.URL,
			CheckStatusCode(nil, nil))
		require.NoError(t, err)
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	})

	t.Run("with failed body writer", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := Get(context.Background(), srv.URL,
			CheckStatusCode(&buf, nil, http.StatusOK))
		require.Error(t, err)
		assert.Contains(t, buf.String(), "accepted")
	})
}

func TestCheckStatusCodeRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok body"))
	}))
	defer srv.Close()

	t.Run("in range", func(t *testing.T) {
		var sc int
		resp, err := Get(context.Background(), srv.URL,
			CheckStatusCodeRange(nil, &sc, 200, 299))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, sc)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("below range", func(t *testing.T) {
		_, err := Get(context.Background(), srv.URL,
			CheckStatusCodeRange(nil, nil, 300, 399))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected response status code")
	})

	t.Run("with failed body writer", func(t *testing.T) {
		var buf bytes.Buffer
		_, err := Get(context.Background(), srv.URL,
			CheckStatusCodeRange(&buf, nil, 300, 399))
		require.Error(t, err)
		assert.Contains(t, buf.String(), "ok body")
	})
}

func TestToString(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response string"))
	}))
	defer srv.Close()

	var s string
	resp, err := Get(context.Background(), srv.URL, ToString(&s))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "response string", s)
}

func TestToBytes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("bytes response"))
	}))
	defer srv.Close()

	t.Run("with n", func(t *testing.T) {
		var n int
		buf := make([]byte, 64)
		resp, err := Get(context.Background(), srv.URL, ToBytes(&n, buf))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "bytes response", string(buf[:n]))
	})

	t.Run("without n", func(t *testing.T) {
		buf := make([]byte, 64)
		resp, err := Get(context.Background(), srv.URL, ToBytes(nil, buf))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "bytes response", string(buf[:14]))
	})
}

func TestToJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(testHandler))
	defer srv.Close()

	var v testRespData
	resp, err := Get(context.Background(), srv.URL, ToJSON(&v))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", v.Message)
}

func TestToWriter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("writer content"))
	}))
	defer srv.Close()

	t.Run("with n", func(t *testing.T) {
		var buf bytes.Buffer
		var n int64
		resp, err := Get(context.Background(), srv.URL, ToWriter(&buf, &n))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "writer content", buf.String())
		assert.Equal(t, int64(14), n)
	})

	t.Run("without n", func(t *testing.T) {
		var buf bytes.Buffer
		resp, err := Get(context.Background(), srv.URL, ToWriter(&buf, nil))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "writer content", buf.String())
	})
}

func TestToAnyDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(testHandler))
	defer srv.Close()

	var v testRespData
	resp, err := Get(context.Background(), srv.URL,
		ToAnyDecode(&v, func(r io.Reader) httpu.Decoder { return json.NewDecoder(r) }))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", v.Message)
}

func TestToAnyUnmarshal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(testHandler))
	defer srv.Close()

	var v testRespData
	resp, err := Get(context.Background(), srv.URL,
		ToAnyUnmarshal(&v, func(bs []byte, v any) error {
			return json.Unmarshal(bs, v)
		}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "ok", v.Message)
}

func TestGuessContentLength(t *testing.T) {
	t.Run("hasLen", func(t *testing.T) {
		l, err := guessContentLength(&hasLenStruct{data: []byte("hello")})
		assert.NoError(t, err)
		assert.Equal(t, int64(5), l)
	})

	t.Run("hasLen2", func(t *testing.T) {
		l, err := guessContentLength(&hasLen2Struct{data: []byte("abcdef")})
		assert.NoError(t, err)
		assert.Equal(t, int64(6), l)
	})

	t.Run("hasSize", func(t *testing.T) {
		l, err := guessContentLength(&hasSizeStruct{data: []byte("xyz")})
		assert.NoError(t, err)
		assert.Equal(t, int64(3), l)
	})

	t.Run("hasSize2", func(t *testing.T) {
		l, err := guessContentLength(&hasSize2Struct{data: []byte("abcd")})
		assert.NoError(t, err)
		assert.Equal(t, int64(4), l)
	})

	t.Run("io.Seeker", func(t *testing.T) {
		r := strings.NewReader("seekme")
		l, err := guessContentLength(r)
		assert.NoError(t, err)
		assert.Equal(t, int64(6), l)
	})

	t.Run("default (no match)", func(t *testing.T) {
		r := strings.NewReader("test")
		// strings.Reader also implements io.Seeker, so wrap it to test default case
		nonSeeker := &nonSeekerReader{Reader: r}
		l, err := guessContentLength(nonSeeker)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), l)
	})
}

type hasLenStruct struct{ data []byte }

func (h *hasLenStruct) Len() int                 { return len(h.data) }
func (h *hasLenStruct) Read([]byte) (int, error) { return 0, io.EOF }

type hasLen2Struct struct{ data []byte }

func (h *hasLen2Struct) Len() int64               { return int64(len(h.data)) }
func (h *hasLen2Struct) Read([]byte) (int, error) { return 0, io.EOF }

type hasSizeStruct struct{ data []byte }

func (h *hasSizeStruct) Size() int                { return len(h.data) }
func (h *hasSizeStruct) Read([]byte) (int, error) { return 0, io.EOF }

type hasSize2Struct struct{ data []byte }

func (h *hasSize2Struct) Size() int64              { return int64(len(h.data)) }
func (h *hasSize2Struct) Read([]byte) (int, error) { return 0, io.EOF }

type nonSeekerReader struct{ io.Reader }

func TestNilOption(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(testHandler))
	defer s.Close()

	// nil options should be skipped without error
	resp, err := Get(context.Background(), s.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDoError(t *testing.T) {
	// Invalid URL scheme should cause an error
	_, err := Get(context.Background(), "://invalid-url")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create http request failed")
}

// errorReadCloser is a reader that returns errors.
type errorReadCloser struct{}

func (e *errorReadCloser) Read(p []byte) (int, error) { return 0, errors.New("read error") }
func (e *errorReadCloser) Close() error               { return nil }

// errorSeeker is a reader+seeker that errors on SeekEnd.
type errorSeeker struct{}

func (e *errorSeeker) Read(p []byte) (int, error) { return 0, io.EOF }
func (e *errorSeeker) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekEnd {
		return 0, errors.New("seek error")
	}
	return 0, nil
}

// errorWriter is a writer that always returns an error.
type errorWriter struct{}

func (e *errorWriter) Write(p []byte) (int, error) { return 0, errors.New("write error") }

// readCloserReader wraps strings.Reader to implement io.ReadCloser.
type readCloserReader struct {
	*strings.Reader
}

func (r *readCloserReader) Close() error { return nil }

// roundTripperFunc is an http.RoundTripper implemented by a function.
type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// okSeeker implements io.Seeker (but not hasLen/hasLen2/hasSize/hasSize2) for testing guessContentLength.
type okSeeker struct {
	size int64
}

func (s *okSeeker) Read(p []byte) (int, error) { return 0, io.EOF }
func (s *okSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekEnd:
		return s.size, nil
	case io.SeekStart:
		return offset, nil
	default:
		return 0, nil
	}
}

// eofWriter returns io.EOF from Write to trigger the defensive e==io.EOF check after io.Copy.
type eofWriter struct{}

func (e *eofWriter) Write(p []byte) (int, error) { return 0, io.EOF }

func TestGuessContentLength_SeekerError(t *testing.T) {
	l, err := guessContentLength(&errorSeeker{})
	assert.Error(t, err)
	assert.Zero(t, l)
}

func TestGuessContentLength_SeekerSuccess(t *testing.T) {
	r := &okSeeker{size: 42}
	l, err := guessContentLength(r)
	assert.NoError(t, err)
	assert.Equal(t, int64(42), l)
}

func TestWithBodyReader_ReadCloser(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		assert.Equal(t, "readcloser body", string(body))
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	resp, err := Post(context.Background(), s.URL,
		WithBodyReader(&readCloserReader{strings.NewReader("readcloser body")}))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestWithBodyReader_GuessLengthError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(testHandler))
	defer s.Close()

	_, err := Post(context.Background(), s.URL,
		WithBodyReader(&errorSeeker{}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "guess content length failed")
}

func TestWithBodyMarshal_Error(t *testing.T) {
	opt := WithBodyMarshal(nil, "application/x-test", func(v any) ([]byte, error) {
		return nil, errors.New("marshal error")
	})
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	err := opt.HandleReq(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "marshal request body failed")
}

func TestWithBodyForm_NilRequest(t *testing.T) {
	opt := WithBodyForm(url.Values{})
	err := opt.HandleReq(nil)
	assert.NoError(t, err)
}

func TestDoWithClientTimeout_ReqHandlerError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(testHandler))
	defer s.Close()

	_, err := DoWithClientTimeout(context.Background(), time.Second, http.MethodGet, s.URL, http.DefaultClient,
		ReqOptionFunc(func(r *http.Request) error {
			return errors.New("req handler error")
		}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handle http request failed")
}

func TestDoWithClientTimeout_RespHandlerError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(testHandler))
	defer s.Close()

	_, err := DoWithClientTimeout(context.Background(), time.Second, http.MethodGet, s.URL, http.DefaultClient,
		RespOptionFunc(func(r *http.Response) error {
			return errors.New("resp handler error")
		}))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "handle http response failed")
}

func TestDoWithClientTimeout_ClientDoError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("transport error")
		}),
	}
	_, err := DoWithClientTimeout(context.Background(), 0, http.MethodGet, "http://example.com", client)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "http request failed")
}

func TestCheckStatusCode_FailedBodyWriterError(t *testing.T) {
	opt := CheckStatusCode(&errorWriter{}, nil, http.StatusOK)
	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("error body")),
	}
	err := opt.HandleResp(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status code")
	assert.Contains(t, err.Error(), "read response body failed")
}

func TestCheckStatusCodeRange_FailedBodyWriterError(t *testing.T) {
	opt := CheckStatusCodeRange(&errorWriter{}, nil, 300, 399)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("ok body")),
	}
	err := opt.HandleResp(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected response status code")
	assert.Contains(t, err.Error(), "read response body failed")
}

func TestToString_ReadError(t *testing.T) {
	var s string
	opt := ToString(&s)
	resp := &http.Response{
		Body: &errorReadCloser{},
	}
	err := opt.HandleResp(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read response body failed")
}

func TestToBytes_ReadError(t *testing.T) {
	buf := make([]byte, 64)
	opt := ToBytes(nil, buf)
	resp := &http.Response{
		Body: &errorReadCloser{},
	}
	err := opt.HandleResp(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read response body failed")
}

func TestToBytes_ReadErrorWithN(t *testing.T) {
	var n int
	buf := make([]byte, 64)
	opt := ToBytes(&n, buf)
	resp := &http.Response{
		Body: &errorReadCloser{},
	}
	err := opt.HandleResp(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read response body failed")
	assert.Zero(t, n)
}

func TestToAnyDecode_DecodeError(t *testing.T) {
	var v testRespData
	opt := ToAnyDecode(&v, func(r io.Reader) httpu.Decoder { return json.NewDecoder(r) })
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("not valid json")),
	}
	err := opt.HandleResp(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode response body failed")
}

func TestToAnyUnmarshal_ReadError(t *testing.T) {
	var v testRespData
	opt := ToAnyUnmarshal(&v, func(bs []byte, v any) error {
		return nil
	})
	resp := &http.Response{
		Body: &errorReadCloser{},
	}
	err := opt.HandleResp(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read response body failed")
}

func TestToAnyUnmarshal_UnmarshalError(t *testing.T) {
	var v testRespData
	opt := ToAnyUnmarshal(&v, func(bs []byte, v any) error {
		return errors.New("unmarshal error")
	})
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("{\"message\":\"ok\"}")),
	}
	err := opt.HandleResp(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal response body failed")
}

func TestToWriter_CopyError(t *testing.T) {
	var n int64
	opt := ToWriter(&errorWriter{}, &n)
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("data")),
	}
	err := opt.HandleResp(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read response body to writer failed")
	assert.Zero(t, n)
}

func TestToWriter_CopyErrorWithoutN(t *testing.T) {
	opt := ToWriter(&errorWriter{}, nil)
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("data")),
	}
	err := opt.HandleResp(resp)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read response body to writer failed")
}

func TestCheckStatusCode_EOFWriter(t *testing.T) {
	opt := CheckStatusCode(&eofWriter{}, nil, http.StatusOK)
	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("body")),
	}
	err := opt.HandleResp(resp)
	assert.Error(t, err)
	// io.Copy returns io.EOF from the writer, so e==io.EOF -> e=nil, no join
	assert.Contains(t, err.Error(), "unexpected response status code")
	assert.NotContains(t, err.Error(), "read response body failed")
}

func TestCheckStatusCodeRange_EOFWriter(t *testing.T) {
	opt := CheckStatusCodeRange(&eofWriter{}, nil, 300, 399)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("body")),
	}
	err := opt.HandleResp(resp)
	assert.Error(t, err)
	// io.Copy returns io.EOF from the writer, so e==io.EOF -> e=nil, no join
	assert.Contains(t, err.Error(), "unexpected response status code")
	assert.NotContains(t, err.Error(), "read response body failed")
}
