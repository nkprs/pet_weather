package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	handlerCfg := httpapi.Config{
		WorkerRPCTimeout: envDuration("GATEWAY_WORKER_RPC_TIMEOUT", 2*time.Second),
		WeatherCacheTTL:  envDuration("GATEWAY_WEATHER_CACHE_TTL", 30*time.Second),
		ForecastCacheTTL: envDuration("GATEWAY_FORECAST_CACHE_TTL", 2*time.Minute),
	}

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

	handler := httpapi.NewHandlerWithConfig(gClient.Weather, handlerCfg)
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
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return def
	}

	v, err := time.ParseDuration(raw)
	if err != nil || v <= 0 {
		slog.Warn("invalid duration env, using default", "key", key, "value", raw, "default", def.String())
		return def
	}
	return v
}
