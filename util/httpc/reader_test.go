package httpc

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/donkeywon/golib/util/httpu"
	"github.com/stretchr/testify/require"
)

var (
	rangeS          *httptest.Server
	noRangeS        *httptest.Server
	downloadContent = []byte("abcdef")
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
	case http.MethodGet:
		w.WriteHeader(http.StatusOK)
		w.Write(downloadContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func setup() {
	rangeS = httptest.NewServer(http.HandlerFunc(rangeDownloadAPI))
	noRangeS = httptest.NewServer(http.HandlerFunc(downloadAPI))
}

func teardown() {
	rangeS.Close()
	noRangeS.Close()
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
	r := NewReader(nil, time.Second, s.URL, PartSize(4))
	defer r.Close()
	bs := make([]byte, 4)
	nr, err := r.Read(bs)
	require.Equal(t, nr, 4)
	require.NoError(t, err)
	nr, err = r.Read(bs)
	require.Equal(t, nr, 2)
	require.ErrorIs(t, err, io.EOF)
}

func TestRangeWriteTo(t *testing.T) {
	testWriteTo(t, rangeS)
}

func TestNoRangeWriteTo(t *testing.T) {
	testWriteTo(t, noRangeS)
}

func testWriteTo(t *testing.T, s *httptest.Server) {
	r := NewReader(nil, time.Second, s.URL, PartSize(4))
	defer r.Close()
	buf := bytes.NewBuffer(nil)
	nr, err := io.Copy(buf, r)
	require.Equal(t, nr, int64(6))
	require.NoError(t, err)
}
