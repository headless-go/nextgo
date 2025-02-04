package http

import "net/http"

type Server interface {
	HandleFunc(method string, path string, handler http.HandlerFunc)
}
