package httpu

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
