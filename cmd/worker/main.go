package main

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	weatherpb "weather/proto"

	"weather/internal/observability"
	handler "weather/internal/worker/grpc"
	"weather/internal/worker/openmeteo"
	"weather/internal/worker/service"

	"buf.build/go/protovalidate"
	protovalidate_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	addr := env("WORKER_GRPC_ADDR", ":50051")
	openMeteoTimeout := envDuration("WORKER_OPENMETEO_TIMEOUT", 2*time.Second)

	otelShutdown, err := observability.Init(context.Background(), "weather-worker")
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

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("listen failed", "err", err)
		return
	}
	defer lis.Close()

	validator, err := protovalidate.New()
	if err != nil {
		slog.Error("validator init failed", "err", err)
		return
	}

	provider := openmeteo.NewProvider(&http.Client{
		Timeout: openMeteoTimeout,
	})

	svc := service.NewWeatherService(provider)
	handler := handler.NewHandler(svc)
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.UnaryInterceptor(protovalidate_middleware.UnaryServerInterceptor(validator)),
	)

	weatherpb.RegisterWeatherServiceServer(grpcServer, handler)

	errCh := make(chan error, 1)
	go func() {
		errCh <- grpcServer.Serve(lis)
	}()

	slog.Info("worker grpc started", "addr", addr)
	select {
	case err := <-errCh:
		if err != nil {
			slog.Error("serve failed", "err", err)
		}
	case <-ctx.Done():
		slog.Info("worker shutdown signal received")
		done := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			slog.Warn("grpc graceful stop timeout; forcing stop")
			grpcServer.Stop()
		}

		select {
		case err := <-errCh:
			if err != nil {
				slog.Error("grpc server exit failed", "err", err)
			}
		case <-time.After(500 * time.Millisecond):
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
