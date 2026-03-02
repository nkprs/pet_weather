package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	weatherpb "weather/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const bufSize = 1024 * 1024

type stubWeatherServer struct {
	weatherpb.UnimplementedWeatherServiceServer
	getCurrent  func(context.Context, *weatherpb.GetCurrentWeatherRequest) (*weatherpb.GetCurrentWeatherResponse, error)
	getForecast func(context.Context, *weatherpb.GetForecastRequest) (*weatherpb.GetForecastResponse, error)
}

func (s *stubWeatherServer) GetCurrentWeather(ctx context.Context, req *weatherpb.GetCurrentWeatherRequest) (*weatherpb.GetCurrentWeatherResponse, error) {
	if s.getCurrent == nil {
		return nil, status.Error(codes.Internal, "not implemented")
	}
	return s.getCurrent(ctx, req)
}

func (s *stubWeatherServer) GetForecast(ctx context.Context, req *weatherpb.GetForecastRequest) (*weatherpb.GetForecastResponse, error) {
	if s.getForecast == nil {
		return nil, status.Error(codes.Internal, "not implemented")
	}
	return s.getForecast(ctx, req)
}

func newGatewayTestServer(t *testing.T, stub *stubWeatherServer) *httptest.Server {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	grpcServer := grpc.NewServer()
	weatherpb.RegisterWeatherServiceServer(grpcServer, stub)
	go func() {
		_ = grpcServer.Serve(lis)
	}()
	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufnet: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	h := NewHandler(weatherpb.NewWeatherServiceClient(conn))
	mux := http.NewServeMux()
	RegisterRoutes(mux, h)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestGetCurrentWeather_OK(t *testing.T) {
	stub := &stubWeatherServer{}
	stub.getCurrent = func(ctx context.Context, req *weatherpb.GetCurrentWeatherRequest) (*weatherpb.GetCurrentWeatherResponse, error) {
		return &weatherpb.GetCurrentWeatherResponse{
			City:         req.GetCity(),
			TemperatureC: -3.2,
			WindSpeedMs:  4.1,
			WeatherCode:  61,
			ObservedAt:   timestamppb.New(time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC)),
		}, nil
	}

	srv := newGatewayTestServer(t, stub)

	res, err := http.Get(srv.URL + "/v1/weather?city=Helsinki")
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var out CurrentWeatherResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if out.City != "Helsinki" || out.TemperatureC != -3.2 || out.WindSpeedMs != 4.1 || out.WeatherCode != 61 {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestGetCurrentWeather_Cache(t *testing.T) {
	var calls int32

	stub := &stubWeatherServer{}
	stub.getCurrent = func(ctx context.Context, gcwr *weatherpb.GetCurrentWeatherRequest) (*weatherpb.GetCurrentWeatherResponse, error) {
		atomic.AddInt32(&calls, 1)
		return &weatherpb.GetCurrentWeatherResponse{
			City:         "Helsinki",
			TemperatureC: -3.2,
			WindSpeedMs:  4.1,
			WeatherCode:  61,
			ObservedAt:   timestamppb.New(time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC)),
		}, nil
	}

	srv := newGatewayTestServer(t, stub)

	doReq := func(path string) CurrentWeatherResponse {
		t.Helper()

		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("http get: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.StatusCode)
		}

		var out CurrentWeatherResponse
		if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
			t.Fatalf("decode: %v", err)
		}

		return out
	}

	first := doReq("/v1/weather?city=Helsinki")
	second := doReq("/v1/weather?city=helsinki")

	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("expected grpc call count = 1, got %d", calls)
	}

	if first != second {
		t.Fatalf("expected cached response to match first response: first=%+v second=%+v", first, second)
	}
}

func TestGetCurrentWeather_GrpcErrors(t *testing.T) {
	cases := []struct {
		name      string
		grpcErr   error
		wantCode  int
		path      string
		cityParam string
	}{
		{"invalid", status.Error(codes.InvalidArgument, "bad"), http.StatusBadRequest, "/v1/weather", "Helsinki"},
		{"notfound", status.Error(codes.NotFound, "no"), http.StatusNotFound, "/v1/weather", "Helsinki"},
		{"timeout", status.Error(codes.DeadlineExceeded, "deadline"), http.StatusGatewayTimeout, "/v1/weather", "Helsinki"},
		{"canceled", status.Error(codes.Canceled, "canceled"), statusClientClosedRequest, "/v1/weather", "Helsinki"},
		{"internal", status.Error(codes.Internal, "boom"), http.StatusServiceUnavailable, "/v1/weather", "Helsinki"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubWeatherServer{}
			stub.getCurrent = func(ctx context.Context, req *weatherpb.GetCurrentWeatherRequest) (*weatherpb.GetCurrentWeatherResponse, error) {
				return nil, tc.grpcErr
			}

			srv := newGatewayTestServer(t, stub)

			res, err := http.Get(srv.URL + tc.path + "?city=" + tc.cityParam)
			if err != nil {
				t.Fatalf("http get: %v", err)
			}
			res.Body.Close()

			if res.StatusCode != tc.wantCode {
				t.Fatalf("expected %d, got %d", tc.wantCode, res.StatusCode)
			}
		})
	}
}

func TestGetCurrentWeather_BadRequest(t *testing.T) {
	stub := &stubWeatherServer{}
	srv := newGatewayTestServer(t, stub)

	res, err := http.Get(srv.URL + "/v1/weather")
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
}

func TestGetForecast_OK(t *testing.T) {
	stub := &stubWeatherServer{}
	stub.getForecast = func(ctx context.Context, req *weatherpb.GetForecastRequest) (*weatherpb.GetForecastResponse, error) {
		if req.GetDays() != 3 {
			t.Fatalf("expected days=3, got %d", req.GetDays())
		}
		return &weatherpb.GetForecastResponse{
			City: req.GetCity(),
			Days: []*weatherpb.DailyForecast{
				{Date: "2026-01-28", TMin: -5, TMax: 1},
				{Date: "2026-01-29", TMin: -3, TMax: 2},
				{Date: "2026-01-30", TMin: -2, TMax: 3},
			},
		}, nil
	}

	srv := newGatewayTestServer(t, stub)

	res, err := http.Get(srv.URL + "/v1/forecast?city=Helsinki&days=3")
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var out ForecastResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if out.City != "Helsinki" || len(out.Days) != 3 {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestGetForecast_BadRequest(t *testing.T) {
	cases := []string{
		"/v1/forecast?city=Helsinki&days=oops",
		"/v1/forecast?city=Helsinki&days=17",
	}

	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			stub := &stubWeatherServer{}
			srv := newGatewayTestServer(t, stub)

			res, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatalf("http get: %v", err)
			}
			defer res.Body.Close()

			if res.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", res.StatusCode)
			}
		})
	}
}

func TestGetForecast_GrpcErrors(t *testing.T) {
	cases := []struct {
		name     string
		grpcErr  error
		wantCode int
	}{
		{"invalid", status.Error(codes.InvalidArgument, "bad"), http.StatusBadRequest},
		{"notfound", status.Error(codes.NotFound, "no"), http.StatusNotFound},
		{"timeout", status.Error(codes.DeadlineExceeded, "deadline"), http.StatusGatewayTimeout},
		{"canceled", status.Error(codes.Canceled, "canceled"), statusClientClosedRequest},
		{"internal", status.Error(codes.Internal, "boom"), http.StatusServiceUnavailable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := &stubWeatherServer{}
			stub.getForecast = func(ctx context.Context, req *weatherpb.GetForecastRequest) (*weatherpb.GetForecastResponse, error) {
				return nil, tc.grpcErr
			}

			srv := newGatewayTestServer(t, stub)

			res, err := http.Get(srv.URL + "/v1/forecast?city=Helsinki&days=2")
			if err != nil {
				t.Fatalf("http get: %v", err)
			}
			res.Body.Close()

			if res.StatusCode != tc.wantCode {
				t.Fatalf("expected %d, got %d", tc.wantCode, res.StatusCode)
			}
		})
	}
}

func TestGetHealthz(t *testing.T) {
	h := &Handler{}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	h.GetHealthz(rr, req)

	res := rr.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}

	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var body map[string]bool
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if !body["ok"] {
		t.Fatalf("expected {\"ok\": true}, got %#v", body)
	}
}
