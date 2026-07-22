package jsons

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testData struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestMarshalUnmarshal(t *testing.T) {
	original := testData{Name: "test", Value: 42}

	bs, err := Marshal(original)
	require.NoError(t, err)
	assert.NotEmpty(t, bs)

	var decoded testData
	err = Unmarshal(bs, &decoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestMarshalStringUnmarshalString(t *testing.T) {
	original := testData{Name: "hello", Value: 99}

	s, err := MarshalString(original)
	require.NoError(t, err)
	assert.NotEmpty(t, s)

	var decoded testData
	err = UnmarshalString(s, &decoded)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestNewEncoder(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	err := enc.Encode(testData{Name: "enc", Value: 1})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "enc")
}

func TestNewDecoder(t *testing.T) {
	body := `{"name":"dec","value":2}`
	dec := NewDecoder(strings.NewReader(body))
	var v testData
	err := dec.Decode(&v)
	require.NoError(t, err)
	assert.Equal(t, "dec", v.Name)
	assert.Equal(t, 2, v.Value)
}

func TestMustMarshal(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		bs := MustMarshal(testData{Name: "must", Value: 7})
		assert.Contains(t, string(bs), "must")
	})

	t.Run("panics on invalid", func(t *testing.T) {
		assert.Panics(t, func() {
			MustMarshal(make(chan int))
		})
	})
}

func TestMustUnmarshal(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		var v testData
		MustUnmarshal([]byte(`{"name":"mustu","value":8}`), &v)
		assert.Equal(t, "mustu", v.Name)
		assert.Equal(t, 8, v.Value)
	})

	t.Run("panics on invalid", func(t *testing.T) {
		assert.Panics(t, func() {
			var v testData
			MustUnmarshal([]byte(`not json`), &v)
		})
	})
}

func TestMustMarshalString(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		s := MustMarshalString(testData{Name: "muststr", Value: 10})
		assert.Contains(t, s, "muststr")
	})

	t.Run("panics on invalid", func(t *testing.T) {
		assert.Panics(t, func() {
			MustMarshalString(make(chan int))
		})
	})
}

func TestMustUnmarshalString(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		var v testData
		MustUnmarshalString(`{"name":"mustustr","value":11}`, &v)
		assert.Equal(t, "mustustr", v.Name)
		assert.Equal(t, 11, v.Value)
	})

	t.Run("panics on invalid", func(t *testing.T) {
		assert.Panics(t, func() {
			var v testData
			MustUnmarshalString("not json", &v)
		})
	})
}

func TestJSONDecoderInterface(t *testing.T) {
	body := `{"name":"iface","value":3}`
	dec := NewDecoder(strings.NewReader(body))

	// Test UseNumber and DisallowUnknownFields are callable
	dec.UseNumber()
	dec.DisallowUnknownFields()

	var v testData
	err := dec.Decode(&v)
	require.NoError(t, err)
	assert.Equal(t, "iface", v.Name)
	assert.Equal(t, 3, v.Value)

	assert.True(t, dec.More() == false) // or just call it
	_ = dec.Buffered()
}

func TestJSONEncoderInterface(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	err := enc.Encode(testData{Name: "enc", Value: 5})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "enc")
}
