package httpu

import (
	"bufio"
	"net"
	"net/http"
)

type RecordResponseWriter struct {
	http.ResponseWriter

	statusCode int
	nw         int
}

func NewRecordResponseWriter(w http.ResponseWriter) *RecordResponseWriter {
	return &RecordResponseWriter{ResponseWriter: w, statusCode: http.StatusOK, nw: -1}
}

func (rp *RecordResponseWriter) Write(data []byte) (int, error) {
	rp.writeHeader()
	nw, err := rp.ResponseWriter.Write(data)
	rp.nw += nw
	return nw, err
}

func (rp *RecordResponseWriter) WriteHeader(statusCode int) {
	if statusCode <= 0 || rp.statusCode == statusCode {
		return
	}
	rp.statusCode = statusCode
}

func (rp *RecordResponseWriter) writeHeader() {
	if rp.Written() {
		rp.nw = 0
		rp.ResponseWriter.WriteHeader(rp.statusCode)
	}
}

func (rp *RecordResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if rp.Written() {
		rp.nw = 0
	}
	return rp.ResponseWriter.(http.Hijacker).Hijack()
}

func (rp *RecordResponseWriter) Flush() {
	rp.writeHeader()
	rp.ResponseWriter.(http.Flusher).Flush()
}

func (rp *RecordResponseWriter) Written() bool {
	return rp.nw != -1
}

func (rp *RecordResponseWriter) Size() int {
	if !rp.Written() {
		return 0
	}
	return rp.nw
}

func (rp *RecordResponseWriter) Status() int {
	return rp.statusCode
}
