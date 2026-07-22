package metricsd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	m := New()
	require.NotNil(t, m)
}

func TestNewCfg(t *testing.T) {
	c := NewCfg()
	assert.False(t, c.EnableProcCollector)
	assert.False(t, c.EnableFDCollector)
	assert.Equal(t, DefaultHTTPEndpointPath, c.HTTPEndpointPath)
}

func TestNewCfgDefaults(t *testing.T) {
	c := NewCfg()
	assert.Equal(t, DefaultEnableProcCollector, c.EnableProcCollector)
	assert.Equal(t, DefaultEnableFDCollector, c.EnableFDCollector)
	assert.Equal(t, DefaultHTTPEndpointPath, c.HTTPEndpointPath)
}

func TestSetCfg(t *testing.T) {
	p := &metricsd{}
	cfg := Cfg{EnableProcCollector: true}
	p.SetCfg(cfg)
	assert.True(t, p.cfg.EnableProcCollector)
}

func TestSetGauge(t *testing.T) {
	p := &metricsd{}
	p.SetGauge("test_setgauge", 42.5)
}

func TestAddGauge(t *testing.T) {
	p := &metricsd{}
	p.AddGauge("test_addgauge", 1.0)
	p.AddGauge("test_addgauge", 2.0)
}

func TestIncGauge(t *testing.T) {
	p := &metricsd{}
	p.IncGauge("test_inc")
}

func TestDecGauge(t *testing.T) {
	p := &metricsd{}
	p.DecGauge("test_dec")
}

func TestIncCounter(t *testing.T) {
	p := &metricsd{}
	p.IncCounter("test_counter")
}

func TestAddCounter(t *testing.T) {
	p := &metricsd{}
	p.AddCounter("test_counter2", 100)
}

func TestAddCountInt64(t *testing.T) {
	p := &metricsd{}
	p.AddCountInt64("test_counter3", 200)
}

func TestMetrics(t *testing.T) {
	p := &metricsd{}
	s := p.Metrics()
	require.NotNil(t, s)
}

func TestHTTPHandler(t *testing.T) {
	p := &metricsd{}
	p.cfg = Cfg{
		EnableProcCollector: false,
		EnableFDCollector:   false,
		HTTPEndpointPath:    "/metrics",
	}

	// Write a known metric so we can verify it appears in the output.
	p.SetGauge("test_http_handler", 99)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	p.httpHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "test_http_handler")
}

func TestHTTPHandler_WithProcCollector(t *testing.T) {
	p := &metricsd{}
	p.cfg = Cfg{
		EnableProcCollector: true,
		EnableFDCollector:   false,
	}

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	p.httpHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// With EnableProcCollector=true, Go runtime metrics should be present.
	assert.Contains(t, rec.Body.String(), "go_goroutines")
}

func TestHTTPHandler_WithFDCollector(t *testing.T) {
	p := &metricsd{}
	p.cfg = Cfg{
		EnableProcCollector: false,
		EnableFDCollector:   true,
	}

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	p.httpHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	// With EnableFDCollector=true, file descriptor metrics should be present.
	assert.Contains(t, rec.Body.String(), "process_open_fds")
}

func TestCompileTimeCheck(t *testing.T) {
	var _ Metricsd = (*metricsd)(nil)
}
