package httpd

import (
	"net/http"
)

type APIHandler func(http.ResponseWriter, *http.Request) []byte

func (ah APIHandler) Handle(w http.ResponseWriter, r *http.Request) []byte {
	return ah(w, r)
}

func (ah APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := sendResponse(ah.Handle(w, r), w, http.StatusOK)
	if err != nil {
		panic(err)
	}
}

func sendResponse(data []byte, w http.ResponseWriter, code int) error {
	w.WriteHeader(code)
	_, err := w.Write(data)
	return err
}
