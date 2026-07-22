package httpc

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/donkeywon/golib/util/httpu"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "test-value", r.Header.Get("test-header"))
		body, _ := io.ReadAll(r.Body)
		require.Equal(t, "abc", string(body))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	}))
	defer s.Close()

	respBody := bytes.NewBuffer(nil)
	_, err := GetTimeout(context.Background(), time.Second, s.URL,
		WithHeaders("test-header", "test-value"),
		WithBody([]byte("abc")),
		CheckStatusCode(respBody, nil, http.StatusOK),
		ToWriter(respBody, nil),
	)
	require.NoError(t, err)
	require.Equal(t, "response", respBody.String())
}

type testReqBody struct {
	FieldA string `json:"fieldA"`
	FieldB int    `json:"fieldB"`
}

func TestPostJSON(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "test-value", r.Header.Get("test-header"))
		require.Equal(t, httpu.MIMEJSON, r.Header.Get("Content-Type"))
		var v testReqBody
		json.NewDecoder(r.Body).Decode(&v)
		require.Equal(t, "abc", v.FieldA)
		require.Equal(t, 123, v.FieldB)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("json response"))
	}))
	defer s.Close()

	respBody := bytes.NewBuffer(nil)
	_, err := PostTimeout(context.Background(), time.Second, s.URL,
		WithHeaders("test-header", "test-value"),
		WithBodyJSON(&testReqBody{FieldA: "abc", FieldB: 123}),
		CheckStatusCode(respBody, nil, http.StatusOK),
		ToWriter(respBody, nil),
	)
	require.NoError(t, err)
	require.Equal(t, "json response", respBody.String())
}

var (
	testAPIResp = []byte("abc")
)

func testAPI(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write(testAPIResp)
}

func BenchmarkHttpc(b *testing.B) {
	s := httptest.NewServer(http.HandlerFunc(testAPI))
	defer s.Close()

	body := []byte("abcdefqweasdzxc")
	buf := bytes.NewBuffer(make([]byte, 64))
	for range b.N {
		buf.Reset()
		PostTimeout(context.Background(), time.Second, s.URL, WithBody(body), WithHeaders("test", "value"), CheckStatusCode(buf, nil, http.StatusOK), ToWriter(buf, nil))
	}
}

func BenchmarkHttp(b *testing.B) {
	s := httptest.NewServer(http.HandlerFunc(testAPI))
	defer s.Close()

	body := []byte("abcdefqweasdzxc")
	buf := bytes.NewBuffer(make([]byte, 64))
	for range b.N {
		func() {
			buf.Reset()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.URL, bytes.NewReader(body))
			if err != nil {
				panic(err)
			}
			req.Header.Set("test", "value")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				panic(err)
			}
			if resp.StatusCode == 200 {

			}
			if resp != nil {
				defer resp.Body.Close()
			}

			io.Copy(buf, resp.Body)
		}()
	}
}
