package server

import (
	"net/http"
	"time"
)

func New(addr string, hdlr http.Handler) *http.Server {
	// TODO: выставлены базовые timeouts
	return &http.Server{
		Addr:              addr,
		Handler:           hdlr,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
