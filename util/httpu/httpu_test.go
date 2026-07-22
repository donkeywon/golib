package httpu

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRespBytes(t *testing.T) {
	w := httptest.NewRecorder()
	RespBytes(w, http.StatusOK, []byte("hello"), "X-Custom", "val")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello", w.Body.String())
	assert.Equal(t, "val", w.Header().Get("X-Custom"))
}

func TestRespBytesPanicsOnWriteError(t *testing.T) {
	assert.Panics(t, func() {
		RespBytes(&panicResponseWriter{}, http.StatusOK, []byte("data"))
	})
}

func TestRespString(t *testing.T) {
	w := httptest.NewRecorder()
	RespString(w, http.StatusCreated, "test string")
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "test string", w.Body.String())
}

func TestRespReader(t *testing.T) {
	w := httptest.NewRecorder()
	r := strings.NewReader("reader content")
	RespReader(w, http.StatusOK, r, "X-Reader", "yes")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "reader content", w.Body.String())
	assert.Equal(t, "yes", w.Header().Get("X-Reader"))
}

func TestRespReaderPanicsOnCopyError(t *testing.T) {
	assert.Panics(t, func() {
		RespReader(&panicResponseWriter{}, http.StatusOK, strings.NewReader("data"))
	})
}

func TestRespJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	RespJSON(w, http.StatusOK, data)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get(HeaderContentType), MIMEJSON)

	var decoded map[string]string
	json.Unmarshal(w.Body.Bytes(), &decoded)
	assert.Equal(t, "value", decoded["key"])
}

func TestRespYAML(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	RespYAML(w, http.StatusOK, data)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get(HeaderContentType), MIMEYAML)
	assert.Contains(t, w.Body.String(), "key")
	assert.Contains(t, w.Body.String(), "value")
}

func TestRespEncoder(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]int{"count": 42}
	RespEncoder(w, http.StatusOK, data, "application/x-custom",
		func(w io.Writer) Encoder { return json.NewEncoder(w) })
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get(HeaderContentType), "application/x-custom")
	assert.Contains(t, w.Body.String(), "42")
}

func TestRespEncoderNilData(t *testing.T) {
	w := httptest.NewRecorder()
	RespEncoder(w, http.StatusOK, nil, MIMEJSON,
		func(w io.Writer) Encoder { return json.NewEncoder(w) })
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRespEncoderPanicsOnEncodeError(t *testing.T) {
	// Writing a channel to JSON encoder will fail
	assert.Panics(t, func() {
		w := httptest.NewRecorder()
		RespEncoder(w, http.StatusOK, make(chan int), MIMEJSON,
			func(w io.Writer) Encoder { return json.NewEncoder(w) })
	})
}

type panicResponseWriter struct{}

func (p *panicResponseWriter) Header() http.Header        { return http.Header{} }
func (p *panicResponseWriter) Write([]byte) (int, error)  { return 0, io.ErrShortWrite }
func (p *panicResponseWriter) WriteHeader(statusCode int) {}

func TestSetHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	setHeaders(w, "A", "1", "B", "2")
	assert.Equal(t, "1", w.Header().Get("A"))
	assert.Equal(t, "2", w.Header().Get("B"))
}

func TestSetHeadersOddCount(t *testing.T) {
	w := httptest.NewRecorder()
	// With odd count, last one is ignored since loop steps by 2
	setHeaders(w, "A", "1", "B")
	assert.Equal(t, "1", w.Header().Get("A"))
	assert.Equal(t, "", w.Header().Get("B"))
}

func TestSetContentTypeHeader(t *testing.T) {
	t.Run("sets when empty", func(t *testing.T) {
		w := httptest.NewRecorder()
		setContentTypeHeader(w, MIMEJSON)
		assert.Equal(t, MIMEJSON, w.Header().Get(HeaderContentType))
	})

	t.Run("does not overwrite existing", func(t *testing.T) {
		w := httptest.NewRecorder()
		w.Header().Set(HeaderContentType, MIMEPlain)
		setContentTypeHeader(w, MIMEJSON)
		assert.Equal(t, MIMEPlain, w.Header().Get(HeaderContentType))
	})
}

func TestReqToJSON(t *testing.T) {
	body := `{"name":"test","value":123}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set(HeaderContentType, MIMEJSON)

	var v struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	err := ReqToJSON(r, &v)
	require.NoError(t, err)
	assert.Equal(t, "test", v.Name)
	assert.Equal(t, 123, v.Value)
}

func TestReqToXML(t *testing.T) {
	type xmlData struct {
		XMLName struct{} `xml:"data"`
		Name    string   `xml:"name"`
		Value   int      `xml:"value"`
	}
	body := `<data><name>xmltest</name><value>456</value></data>`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set(HeaderContentType, MIMEXML)

	var v xmlData
	err := ReqToXML(r, &v)
	require.NoError(t, err)
	assert.Equal(t, "xmltest", v.Name)
	assert.Equal(t, 456, v.Value)
}

func TestReqToYAML(t *testing.T) {
	body := "name: yamltest\nvalue: 789\n"
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	r.Header.Set(HeaderContentType, MIMEYAML)

	var v struct {
		Name  string `yaml:"name"`
		Value int    `yaml:"value"`
	}
	err := ReqToYAML(r, &v)
	require.NoError(t, err)
	assert.Equal(t, "yamltest", v.Name)
	assert.Equal(t, 789, v.Value)
}

func TestReqTo(t *testing.T) {
	type data struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	t.Run("JSON content type", func(t *testing.T) {
		body := `{"name":"json","value":1}`
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set(HeaderContentType, MIMEJSON)
		var v data
		err := ReqTo(r, &v)
		require.NoError(t, err)
		assert.Equal(t, "json", v.Name)
	})

	t.Run("JSON UTF8 content type", func(t *testing.T) {
		body := `{"name":"json2","value":2}`
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set(HeaderContentType, MIMEJSONUTF8)
		var v data
		err := ReqTo(r, &v)
		require.NoError(t, err)
		assert.Equal(t, "json2", v.Name)
	})

	t.Run("XML content type", func(t *testing.T) {
		body := `<data><name>xmlval</name><value>42</value></data>`
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set(HeaderContentType, MIMEXML)
		var v struct {
			XMLName struct{} `xml:"data"`
			Name    string   `xml:"name"`
			Value   int      `xml:"value"`
		}
		err := ReqTo(r, &v)
		require.NoError(t, err)
		assert.Equal(t, "xmlval", v.Name)
		assert.Equal(t, 42, v.Value)
	})

	t.Run("XML2 content type", func(t *testing.T) {
		body := `<data><name>xml2val</name><value>7</value></data>`
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set(HeaderContentType, MIMEXML2)
		var v struct {
			XMLName struct{} `xml:"data"`
			Name    string   `xml:"name"`
			Value   int      `xml:"value"`
		}
		err := ReqTo(r, &v)
		require.NoError(t, err)
		assert.Equal(t, "xml2val", v.Name)
	})

	t.Run("YAML content type", func(t *testing.T) {
		body := "name: yamlval\nvalue: 99\n"
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set(HeaderContentType, MIMEYAML)
		var v struct {
			Name  string `yaml:"name"`
			Value int    `yaml:"value"`
		}
		err := ReqTo(r, &v)
		require.NoError(t, err)
		assert.Equal(t, "yamlval", v.Name)
		assert.Equal(t, 99, v.Value)
	})

	t.Run("YAML2 content type", func(t *testing.T) {
		body := "name: yaml2val\nvalue: 88\n"
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set(HeaderContentType, MIMEYAML2)
		var v struct {
			Name  string `yaml:"name"`
			Value int    `yaml:"value"`
		}
		err := ReqTo(r, &v)
		require.NoError(t, err)
		assert.Equal(t, "yaml2val", v.Name)
	})

	t.Run("unknown content type defaults to JSON", func(t *testing.T) {
		body := `{"name":"default","value":3}`
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		r.Header.Set(HeaderContentType, "text/plain")
		var v data
		err := ReqTo(r, &v)
		require.NoError(t, err)
		assert.Equal(t, "default", v.Name)
	})

	t.Run("empty body returns nil (EOF handled)", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
		r.Header.Set(HeaderContentType, MIMEJSON)
		var v data
		err := ReqTo(r, &v)
		assert.NoError(t, err)
	})
}

func TestGetRealRemoteIP(t *testing.T) {
	t.Run("RemoteAddr only", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "192.168.1.1:12345"
		ip := GetRealRemoteIP(r)
		assert.Equal(t, "192.168.1.1", ip)
	})

	t.Run("X-Forwarded-For", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:12345"
		r.Header.Set("X-Forwarded-For", "192.168.1.100")
		ip := GetRealRemoteIP(r)
		assert.Equal(t, "192.168.1.100", ip)
	})

	t.Run("X-Forwarded-For with multiple IPs", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Forwarded-For", "192.168.1.10, 10.0.0.1")
		ip := GetRealRemoteIP(r)
		assert.NotEmpty(t, ip)
	})

	t.Run("X-Real-Ip", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "1.1.1.1:999"
		r.Header.Set("X-Real-Ip", "172.16.0.1")
		ip := GetRealRemoteIP(r)
		assert.Equal(t, "172.16.0.1", ip)
	})

	t.Run("X-Real-Ip takes priority when no X-Forwarded-For", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "1.1.1.1:999"
		r.Header.Set("X-Real-Ip", "10.10.10.10")
		ip := GetRealRemoteIP(r)
		assert.Equal(t, "10.10.10.10", ip)
	})

	t.Run("X-Forwarded-For takes priority over X-Real-Ip", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Forwarded-For", "192.168.1.200")
		r.Header.Set("X-Real-Ip", "10.10.10.10")
		ip := GetRealRemoteIP(r)
		assert.Equal(t, "192.168.1.200", ip)
	})

	t.Run("invalid IP in headers falls back to RemoteAddr", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.20.30.40:5555"
		r.Header.Set("X-Forwarded-For", "not-an-ip")
		ip := GetRealRemoteIP(r)
		assert.Equal(t, "10.20.30.40", ip)
	})

	t.Run("empty remote addr", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = ""
		ip := GetRealRemoteIP(r)
		assert.Equal(t, "", ip)
	})

	t.Run("Forwarded-For with trailing comma", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "10.0.0.1:12345"
		r.Header.Set("X-Forwarded-For", "192.168.1.100,")
		ip := GetRealRemoteIP(r)
		// Trailing comma is trimmed, so it parses
		assert.Equal(t, "192.168.1.100", ip)
	})
}
