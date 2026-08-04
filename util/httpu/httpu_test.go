package httpu

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRealRemoteIP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header http.Header
		remote string
		want   string
	}{
		{
			name:   "remote addr ipv4",
			remote: "1.2.3.4:8080",
			want:   "1.2.3.4",
		},
		{
			name:   "remote addr ipv6",
			remote: "[::1]:8080",
			want:   "::1",
		},
		{
			name:   "xff takes leftmost entry",
			header: http.Header{HeaderXForwardedFor: []string{"1.2.3.4, 5.6.7.8"}},
			remote: "9.9.9.9:80",
			want:   "1.2.3.4",
		},
		{
			name:   "xff skips invalid entries",
			header: http.Header{HeaderXForwardedFor: []string{"not-an-ip, 5.6.7.8"}},
			remote: "9.9.9.9:80",
			want:   "5.6.7.8",
		},
		{
			name:   "xff invalid falls back to x-real-ip",
			header: http.Header{HeaderXForwardedFor: []string{"not-an-ip"}, HeaderXRealIP: []string{"5.6.7.8"}},
			remote: "9.9.9.9:80",
			want:   "5.6.7.8",
		},
		{
			name:   "x-real-ip",
			header: http.Header{HeaderXRealIP: []string{"5.6.7.8"}},
			remote: "9.9.9.9:80",
			want:   "5.6.7.8",
		},
		{
			name:   "no headers falls back to remote addr",
			remote: "9.9.9.9:80",
			want:   "9.9.9.9",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &http.Request{Header: tt.header, RemoteAddr: tt.remote}
			assert.Equal(t, tt.want, GetRealRemoteIP(r))
		})
	}
}

func TestRespJSON(t *testing.T) {
	t.Parallel()

	t.Run("sets content type and body", func(t *testing.T) {
		t.Parallel()
		w := httptest.NewRecorder()
		RespJSON(w, http.StatusCreated, map[string]string{"a": "b"})
		assert.Equal(t, http.StatusCreated, w.Code)
		assert.Contains(t, w.Header().Get(HeaderContentType), MIMEJSON)
		assert.JSONEq(t, `{"a":"b"}`, w.Body.String())
	})
}
