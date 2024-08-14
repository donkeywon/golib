package httpd

import (
	"bufio"
	"net"
	"net/http"
)

type recordResponseWriter struct {
	http.ResponseWriter

	statusCode int
	nw         int
}

func newWriteOnceRecordResponseWriter(w http.ResponseWriter) *recordResponseWriter {
	return &recordResponseWriter{
		ResponseWriter: w,
		nw:             -1,
		statusCode:     http.StatusOK,
	}
}

func (rp *recordResponseWriter) Write(data []byte) (int, error) {
	rp.writeHeader()
	nw, err := rp.ResponseWriter.Write(data)
	rp.nw += nw
	return nw, err
}

func (rp *recordResponseWriter) WriteHeader(statusCode int) {
	if statusCode <= 0 || rp.statusCode == statusCode {
		return
	}
	rp.statusCode = statusCode
}

func (rp *recordResponseWriter) writeHeader() {
	if rp.nw == -1 {
		rp.nw = 0
		rp.ResponseWriter.WriteHeader(rp.statusCode)
	}
}

func (rp *recordResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if rp.nw == -1 {
		rp.nw = 0
	}
	return rp.ResponseWriter.(http.Hijacker).Hijack()
}

func (rp *recordResponseWriter) Flush() {
	rp.writeHeader()
	rp.ResponseWriter.(http.Flusher).Flush()
}
