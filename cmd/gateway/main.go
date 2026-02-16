package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"weather/internal/gateway/httpapi"
	"weather/internal/gateway/rpc"
	"weather/internal/gateway/server"
	"weather/internal/observability"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpAddr := env("GATEWAY_HTTP_ADDR", ":8081")
	workerAddr := env("WORKER_GRPC_ADDR", "127.0.0.1:50051")

	otelShutdown, err := observability.Init(context.Background(), "weather-gateway")
	if err != nil {
		slog.Error("otel init failed", "err", err)
		return
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelShutdown(shutdownCtx); err != nil {
			slog.Error("otel shutdown failed", "err", err)
		}
	}()

	gClient, err := rpc.NewClient(workerAddr)
	if err != nil {
		slog.Error("grpc client failed", "err", err)
		return
	}
	defer gClient.Close()

	mux := http.NewServeMux()

	handler := httpapi.NewHandler(gClient.Weather)
	httpapi.RegisterRoutes(mux, handler)

	otelHandler := otelhttp.NewHandler(mux, "gateway-http")
	srv := server.New(httpAddr, server.LoggingMw(otelHandler))

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	slog.Info("gateway http started", "addr", httpAddr)
	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("http serve failed", "err", err)
		}
	case <-ctx.Done():
		slog.Info("gateway shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("http shutdown failed", "err", err)
		}

		if err := <-errCh; err != nil && err != http.ErrServerClosed {
			slog.Error("http server exit failed", "err", err)
		}
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
