package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
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
		return &weatherpb.GetForecastResponse{
			City: req.GetCity(),
			Days: []*weatherpb.DailyForecast{
				{Date: "2026-01-28", TMin: -5, TMax: 1},
				{Date: "2026-01-29", TMin: -3, TMax: 2},
			},
		}, nil
	}

	srv := newGatewayTestServer(t, stub)

	res, err := http.Get(srv.URL + "/v1/forecast?city=Helsinki&days=2")
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

	if out.City != "Helsinki" || len(out.Days) != 2 {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestGetForecast_BadRequest(t *testing.T) {
	stub := &stubWeatherServer{}
	srv := newGatewayTestServer(t, stub)

	res, err := http.Get(srv.URL + "/v1/forecast?city=Helsinki&days=oops")
	if err != nil {
		t.Fatalf("http get: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
}
