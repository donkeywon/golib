package httpc

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	respBody := bytes.NewBuffer(nil)
	reqBody := []byte("abc")
	_, err := Get(nil, time.Second, "http://127.0.0.1:8085/get",
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
	_, err := Post(nil, time.Second, "http://127.0.0.1:8085/post",
		WithHeaders("test-header", "test-value"),
		WithBodyJSON(reqBody),
		CheckStatusCode(http.StatusOK),
		ToBytesBuffer(respBody),
	)

	require.NoError(t, err)
	fmt.Println(respBody.String())
}
