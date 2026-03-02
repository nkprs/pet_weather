package rpc

import "testing"

func TestNewClient_OK(t *testing.T) {
	c, err := NewClient("127.0.0.1:40041")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Weather == nil {
		t.Fatal("expected non-nil Weather client")
	}
	if c.conn == nil {
		t.Fatal("expected non-nil grpc conn")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestNewClient_InvalidTarget(t *testing.T) {
	c, err := NewClient("dns:///%zz")
	if err == nil {
		if c != nil {
			_ = c.Close()
		}
		t.Fatal("expected error, got nil")
	}
	if c != nil {
		t.Fatal("expected nil client on error")
	}
}
