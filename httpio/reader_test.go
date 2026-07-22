package httpio

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/donkeywon/golib/util/httpc"
	"github.com/donkeywon/golib/util/httpu"
	"github.com/stretchr/testify/require"
)

var (
	rangeS           *httptest.Server
	noRangeS         *httptest.Server
	errorHeadS       *httptest.Server
	errorRangeGetS   *httptest.Server
	noRangeErrorGetS *httptest.Server
	downloadContent  = []byte("abcdef")
)

func rangeDownloadAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodHead:
		w.Header().Set(httpu.HeaderAcceptRanges, "bytes")
		w.Header().Set(httpu.HeaderContentLength, strconv.Itoa(len(downloadContent)))
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		rangeHeader := r.Header.Get("Range")
		rangeBytes := strings.SplitN(rangeHeader, "=", 2)
		ranges := strings.SplitN(rangeBytes[1], "-", 2)
		start, startErr := strconv.Atoi(ranges[0])
		end, endErr := strconv.Atoi(ranges[1])
		if startErr != nil || endErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(errors.Join(startErr, endErr).Error()))
			return
		}

		if start >= len(downloadContent) {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		end = min(end+1, len(downloadContent))
		w.WriteHeader(http.StatusOK)
		w.Write(downloadContent[start:end])
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func downloadAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodHead:
		w.Header().Set(httpu.HeaderContentLength, strconv.Itoa(len(downloadContent)))
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		w.WriteHeader(http.StatusOK)
		w.Write(downloadContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func errorHeadAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodHead:
		w.WriteHeader(http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func errorRangeGetAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodHead:
		w.Header().Set(httpu.HeaderAcceptRanges, "bytes")
		w.Header().Set(httpu.HeaderContentLength, strconv.Itoa(len(downloadContent)))
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		w.WriteHeader(http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func noRangeErrorGetAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodHead:
		w.Header().Set(httpu.HeaderContentLength, strconv.Itoa(len(downloadContent)))
		w.WriteHeader(http.StatusOK)
	case http.MethodGet:
		w.WriteHeader(http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func setup() {
	rangeS = httptest.NewServer(http.HandlerFunc(rangeDownloadAPI))
	noRangeS = httptest.NewServer(http.HandlerFunc(downloadAPI))
	errorHeadS = httptest.NewServer(http.HandlerFunc(errorHeadAPI))
	errorRangeGetS = httptest.NewServer(http.HandlerFunc(errorRangeGetAPI))
	noRangeErrorGetS = httptest.NewServer(http.HandlerFunc(noRangeErrorGetAPI))
}

func teardown() {
	rangeS.Close()
	noRangeS.Close()
	errorHeadS.Close()
	errorRangeGetS.Close()
	noRangeErrorGetS.Close()
}

func TestMain(m *testing.M) {
	setup()
	exit := m.Run()
	teardown()
	os.Exit(exit)
}

func TestRangeRead(t *testing.T) {
	testRead(t, rangeS)
}

func TestNoRangeRead(t *testing.T) {
	testRead(t, noRangeS)
}

func testRead(t *testing.T, s *httptest.Server) {
	r := NewReader(context.TODO(), s.URL)
	defer r.Close()
	bs := make([]byte, 4)
	nr, err := r.Read(bs)
	require.NoError(t, err)
	require.Equal(t, 4, nr)
	nr, err = r.Read(bs)
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, 2, nr)
}

func TestRangeWriteTo(t *testing.T) {
	testWriteTo(t, rangeS)
}

func TestNoRangeWriteTo(t *testing.T) {
	testWriteTo(t, noRangeS)
}

func testWriteTo(t *testing.T, s *httptest.Server) {
	r := NewReader(context.TODO(), s.URL)
	defer r.Close()
	buf := bytes.NewBuffer(nil)
	nr, err := io.Copy(buf, r)
	require.Equal(t, int64(6), nr)
	require.NoError(t, err)
}

// TestReadAt with range server
func TestReadAt_Range(t *testing.T) {
	r := NewReader(context.Background(), rangeS.URL)
	defer r.Close()
	p := make([]byte, 4)
	nr, err := r.ReadAt(p, 0)
	require.NoError(t, err)
	require.Equal(t, 4, nr)
	require.Equal(t, "abcd", string(p[:nr]))
}

// TestReadAt with no-range server (returns ErrRangeUnsupported)
func TestReadAt_NoRange(t *testing.T) {
	r := NewReader(context.Background(), noRangeS.URL)
	defer r.Close()
	_, err := r.ReadAt(make([]byte, 4), 0)
	require.ErrorIs(t, err, ErrRangeUnsupported)
}

// TestReadAt with cancelled context
func TestReadAt_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := NewReader(ctx, rangeS.URL)
	defer r.Close()
	_, err := r.ReadAt(make([]byte, 4), 0)
	require.ErrorIs(t, err, context.Canceled)
}

// TestReadAt with empty buffer
func TestReadAt_EmptyBuffer(t *testing.T) {
	r := NewReader(context.Background(), rangeS.URL)
	defer r.Close()
	nr, err := r.ReadAt([]byte{}, 0)
	require.NoError(t, err)
	require.Equal(t, 0, nr)
}

// TestLen
func TestLen(t *testing.T) {
	t.Run("end <= 0 returns -1", func(t *testing.T) {
		r := NewReader(context.Background(), rangeS.URL)
		defer r.Close()
		r.end = 0
		require.Equal(t, int64(-1), r.Len())
	})
	t.Run("with known end", func(t *testing.T) {
		r := &Reader{end: 10, offset: 3}
		require.Equal(t, int64(7), r.Len())
	})
}

// TestSize
func TestSize(t *testing.T) {
	t.Run("end <= 0 returns -1", func(t *testing.T) {
		r := &Reader{end: 0}
		require.Equal(t, int64(-1), r.Size())
	})
	t.Run("with known end and offset", func(t *testing.T) {
		r := &Reader{end: 10, opt: &option{offset: 5}}
		require.Equal(t, int64(5), r.Size())
	})
}

// TestRead with cancelled context
func TestRead_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := NewReader(ctx, rangeS.URL)
	defer r.Close()
	_, err := r.Read(make([]byte, 4))
	require.ErrorIs(t, err, context.Canceled)
}

// TestRead with empty buffer
func TestRead_EmptyBuffer(t *testing.T) {
	r := NewReader(context.Background(), rangeS.URL)
	defer r.Close()
	nr, err := r.Read([]byte{})
	require.NoError(t, err)
	require.Equal(t, 0, nr)
}

// TestWriteTo with cancelled context
func TestWriteTo_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := NewReader(ctx, rangeS.URL)
	defer r.Close()
	_, err := r.WriteTo(bytes.NewBuffer(nil))
	require.ErrorIs(t, err, context.Canceled)
}

// TestRead_NoRange - covers the non-range read path
func TestRead_NoRange(t *testing.T) {
	r := NewReader(context.Background(), noRangeS.URL)
	defer r.Close()
	bs := make([]byte, 3)
	nr, err := r.Read(bs)
	require.NoError(t, err)
	require.Equal(t, 3, nr)
	require.Equal(t, "abc", string(bs[:nr]))
	nr, err = r.Read(bs)
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, 3, nr)
	require.Equal(t, "def", string(bs[:nr]))
}

// TestWriteTo_NoRange - covers the non-range WriteTo path
func TestWriteTo_NoRange(t *testing.T) {
	r := NewReader(context.Background(), noRangeS.URL)
	defer r.Close()
	buf := bytes.NewBuffer(nil)
	nw, err := r.WriteTo(buf)
	require.NoError(t, err)
	require.Equal(t, int64(6), nw)
	require.Equal(t, "abcdef", buf.String())
}

// TestClose
func TestClose(t *testing.T) {
	r := NewReader(context.Background(), rangeS.URL)
	// Don't read anything, just close.
	err := r.Close()
	require.NoError(t, err)
	// Second close should be fine (sync.Once)
	err = r.Close()
	require.NoError(t, err)
}

// TestReaderOffset
func TestReaderOffset(t *testing.T) {
	r := NewReader(context.Background(), rangeS.URL)
	defer r.Close()
	require.Equal(t, int64(0), r.Offset())
	// Do a read
	p := make([]byte, 3)
	r.Read(p)
	require.Equal(t, int64(3), r.Offset())
}

// TestNewReader_WithClient
func TestNewReader_WithClient(t *testing.T) {
	c := &http.Client{}
	r := NewReader(context.Background(), rangeS.URL, WithClient(c))
	defer r.Close()
	require.NotNil(t, r.c)
}

// TestNewReader_WithLimit
func TestNewReader_WithLimit(t *testing.T) {
	r := NewReader(context.Background(), rangeS.URL, Limit(4))
	defer r.Close()
	require.Equal(t, int64(4), r.end)
}

// TestNewReader_WithOffset
func TestNewReader_WithOffset(t *testing.T) {
	r := NewReader(context.Background(), rangeS.URL, Offset(2))
	defer r.Close()
	require.Equal(t, int64(2), r.Offset())
}

// TestNewReader_NilContext covers the nil context panic path in NewReader
func TestNewReader_NilContext(t *testing.T) {
	require.PanicsWithValue(t, "nil context", func() {
		NewReader(nil, "http://example.com")
	})
}

// TestNewReader_WithResponseHeaderTimeout covers the responseHeaderTimeout > 0 path
func TestNewReader_WithResponseHeaderTimeout(t *testing.T) {
	r := NewReader(context.Background(), rangeS.URL, WithResponseHeaderTimeout(5*time.Second))
	defer r.Close()
	require.NotNil(t, r.c)
}

// TestNewReader_WithHTTPOptions
func TestNewReader_WithHTTPOptions(t *testing.T) {
	r := NewReader(context.Background(), rangeS.URL, WithHTTPOptions(
		httpc.WithHeaders("Authorization", "Bearer test")))
	defer r.Close()
	require.Len(t, r.opt.httpOptions, 1)
}

// TestHead_Error_Read covers the head error path in Read
func TestHead_Error_Read(t *testing.T) {
	r := NewReader(context.Background(), errorHeadS.URL)
	defer r.Close()
	_, err := r.Read(make([]byte, 4))
	require.Error(t, err)
	require.Contains(t, err.Error(), "head failed")
}

// TestHead_Error_ReadAt covers the head error path in ReadAt
func TestHead_Error_ReadAt(t *testing.T) {
	r := NewReader(context.Background(), errorHeadS.URL)
	defer r.Close()
	_, err := r.ReadAt(make([]byte, 4), 0)
	require.Error(t, err)
	require.Contains(t, err.Error(), "head failed")
}

// TestHead_Error_WriteTo covers the head error path in WriteTo
func TestHead_Error_WriteTo(t *testing.T) {
	r := NewReader(context.Background(), errorHeadS.URL)
	defer r.Close()
	_, err := r.WriteTo(bytes.NewBuffer(nil))
	require.Error(t, err)
	require.Contains(t, err.Error(), "head failed")
}

// TestRetryGetRemain covers the retryGetRemain function
func TestRetryGetRemain(t *testing.T) {
	r := NewReader(context.Background(), rangeS.URL)
	defer r.Close()
	_ = r.init()
	body, err := r.retryGetRemain()
	require.NoError(t, err)
	require.NotNil(t, body)
	body.Close()
}

// TestRemainWriteTo_GetError covers remainWriteTo error from getRemain and
// the respBody-close-on-error path in getRemain
func TestRemainWriteTo_GetError(t *testing.T) {
	r := NewReader(context.Background(), errorRangeGetS.URL)
	defer r.Close()
	_, err := r.WriteTo(bytes.NewBuffer(nil))
	require.Error(t, err)
}

// TestReadFromRemain_GetError covers readFromRemain error from getRemain
func TestReadFromRemain_GetError(t *testing.T) {
	r := NewReader(context.Background(), errorRangeGetS.URL)
	defer r.Close()
	_, err := r.Read(make([]byte, 4))
	require.Error(t, err)
}

// TestRead_NoRangeGetError covers retryGetNoRange's error and respBody-close paths
func TestRead_NoRangeGetError(t *testing.T) {
	r := NewReader(context.Background(), noRangeErrorGetS.URL)
	defer r.Close()
	_, err := r.Read(make([]byte, 4))
	require.Error(t, err)
}

// TestGetRemain_CloseBodyOnError covers the respBody.Close() path in getRemain.
// Injects a failing CheckStatusCode via httpOptions that runs after RespOptionFunc
// captures the body, so respBody is non-nil when the error occurs.
func TestGetRemain_CloseBodyOnError(t *testing.T) {
	r := NewReader(context.Background(), rangeS.URL)
	defer r.Close()
	require.NoError(t, r.init())
	// Append a CheckStatusCode that fails for the server's 200 response.
	r.opt.httpOptions = append(r.opt.httpOptions,
		httpc.CheckStatusCode(nil, nil, http.StatusTeapot))
	_, err := r.getRemain()
	require.Error(t, err)
}

// TestRetryGetNoRange_CloseBodyOnError covers the respBody.Close() path in retryGetNoRange.
func TestRetryGetNoRange_CloseBodyOnError(t *testing.T) {
	r := NewReader(context.Background(), noRangeS.URL)
	defer r.Close()
	require.NoError(t, r.init())
	r.opt.httpOptions = append(r.opt.httpOptions,
		httpc.CheckStatusCode(nil, nil, http.StatusTeapot))
	_, err := r.retryGetNoRange()
	require.Error(t, err)
}

// TestReadFromRemain_ReadError covers the body-read non-EOF error path in readFromRemain.
// We directly set an erroring reader as respBody to simulate a mid-read network error.
func TestReadFromRemain_ReadError(t *testing.T) {
	r := NewReader(context.Background(), rangeS.URL)
	defer r.Close()
	require.NoError(t, r.init())

	// Directly inject an erroring respBodyReader to trigger the non-EOF error path.
	r.respBody = &respBodyReader{
		ReadCloser: &errorReader{data: "ab", failAfterRead: true},
		r:          r,
	}

	_, err := r.readFromRemain(make([]byte, 4))
	require.Error(t, err)
	require.NotErrorIs(t, err, io.EOF)
	require.Nil(t, r.respBody) // should be cleared on non-EOF error
}

// errorReader is an io.ReadCloser that returns an error after being read.
type errorReader struct {
	data          string
	pos           int
	failAfterRead bool
}

func (e *errorReader) Read(p []byte) (int, error) {
	if e.failAfterRead && e.pos > 0 {
		return 0, errors.New("injected read error")
	}
	if e.pos >= len(e.data) {
		return 0, io.EOF
	}
	n := copy(p, e.data[e.pos:])
	e.pos += n
	return n, nil
}

func (e *errorReader) Close() error { return nil }
