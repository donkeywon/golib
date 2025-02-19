package httpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	respBody := bytes.NewBuffer(nil)
	reqBody := []byte("abc")
	_, err := Get(context.Background(), time.Second, "http://127.0.0.1:8085/get",
		WithHeaders("test-header", "test-value"),
		WithBody(reqBody),
		CheckStatusCode(http.StatusOK),
		ToBytesBuffer(respBody),
	)

	require.NoError(t, err)
	fmt.Println(respBody.String())
}

type testReqBody struct {
	FieldA string `json:"fieldA"`
	FieldB int    `json:"fieldB"`
}

func TestPostJSON(t *testing.T) {
	respBody := bytes.NewBuffer(nil)
	reqBody := &testReqBody{
		FieldA: "abc",
		FieldB: 123,
	}
	_, err := Post(context.Background(), time.Second, "http://127.0.0.1:8085/post",
		WithHeaders("test-header", "test-value"),
		WithBodyJSON(reqBody),
		CheckStatusCode(http.StatusOK),
		ToBytesBuffer(respBody),
	)

	require.NoError(t, err)
	fmt.Println(respBody.String())
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
		Post(context.Background(), time.Second, s.URL, WithBody(body), WithHeaders("test", "value"), CheckStatusCode(200), ToWriter(nil, buf))
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
