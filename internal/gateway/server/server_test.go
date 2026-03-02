package server

import (
	"net/http"
	"testing"
)

func TestNewServer_OK(t *testing.T) {
	addr := "127.0.0.1:3000"
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	srv := New(addr, h)

	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv.Addr != addr {
		t.Fatalf("expected addr %q, got %q", addr, srv.Addr)
	}
	if srv.Handler == nil {
		t.Fatal("expected handler to be set")
	}
	if srv.ReadHeaderTimeout != readHeaderTimeout {
		t.Fatalf("expected ReadHeaderTimeout %v, got %v", readHeaderTimeout, srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != readTimeout {
		t.Fatalf("expected ReadTimeout %v, got %v", readTimeout, srv.ReadTimeout)
	}
	if srv.WriteTimeout != writeTimeout {
		t.Fatalf("expected WriteTimeout %v, got %v", writeTimeout, srv.WriteTimeout)
	}
	if srv.IdleTimeout != idleTimeout {
		t.Fatalf("expected IdleTimeout %v, got %v", idleTimeout, srv.IdleTimeout)
	}
}
