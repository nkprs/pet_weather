package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"weather/internal/gateway/cache"
	weatherpb "weather/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const statusClientClosedRequest = 499

const (
	defaultWorkerRPCTimeout = 2 * time.Second
	defaultWeatherCacheTTL  = 30 * time.Second
	defaultForecastCacheTTL = 2 * time.Minute
)

type Config struct {
	WorkerRPCTimeout time.Duration
	WeatherCacheTTL  time.Duration
	ForecastCacheTTL time.Duration
}

func (c Config) normalized() Config {
	if c.WorkerRPCTimeout <= 0 {
		c.WorkerRPCTimeout = defaultWorkerRPCTimeout
	}
	if c.WeatherCacheTTL <= 0 {
		c.WeatherCacheTTL = defaultWeatherCacheTTL
	}
	if c.ForecastCacheTTL <= 0 {
		c.ForecastCacheTTL = defaultForecastCacheTTL
	}
	return c
}

type Handler struct {
	grcp             weatherpb.WeatherServiceClient
	workerRPCTimeout time.Duration
	weatherCache     *cache.Cache[CurrentWeatherResponse]
	forecastCache    *cache.Cache[ForecastResponse]
}

func NewHandler(client weatherpb.WeatherServiceClient) *Handler {
	return NewHandlerWithConfig(client, Config{})
}

func NewHandlerWithConfig(client weatherpb.WeatherServiceClient, cfg Config) *Handler {
	cfg = cfg.normalized()

	weatherCache := cache.New[CurrentWeatherResponse](cfg.WeatherCacheTTL)
	forecastCache := cache.New[ForecastResponse](cfg.ForecastCacheTTL)
	return &Handler{
		grcp:             client,
		workerRPCTimeout: cfg.WorkerRPCTimeout,
		weatherCache:     weatherCache,
		forecastCache:    forecastCache,
	}
}

func (h *Handler) GetHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{
		"ok": true,
	})
}

func (h *Handler) GetCurrentWeather(w http.ResponseWriter, r *http.Request) {
	q := CurrentWeatherQuery{City: r.URL.Query().Get("city")}
	if err := validate.Struct(q); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	key := "weather:" + strings.ToLower(q.City)
	if v, ok := h.weatherCache.Get(key); ok {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.workerRPCTimeout)
	defer cancel()

	resp, err := h.grcp.GetCurrentWeather(ctx, &weatherpb.GetCurrentWeatherRequest{
		City: q.City,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.InvalidArgument:
				w.WriteHeader(http.StatusBadRequest)
			case codes.NotFound:
				w.WriteHeader(http.StatusNotFound)
			case codes.DeadlineExceeded:
				w.WriteHeader(http.StatusGatewayTimeout)
			case codes.Canceled:
				w.WriteHeader(statusClientClosedRequest)
			default:
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	out := CurrentWeatherResponse{
		City:         resp.City,
		TemperatureC: resp.TemperatureC,
		WindSpeedMs:  resp.WindSpeedMs,
		WeatherCode:  resp.WeatherCode,
		ObservedAt:   resp.ObservedAt.AsTime().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
	h.weatherCache.Set(key, out)
}

func (h *Handler) GetForecast(w http.ResponseWriter, r *http.Request) {
	city := r.URL.Query().Get("city")
	daysStr := r.URL.Query().Get("days")

	days64, err := strconv.ParseInt(daysStr, 10, 32)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	q := ForecastQuery{City: city, Days: int32(days64)}
	if err := validate.Struct(q); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	key := "forecast:" + strings.ToLower(q.City) + ":" + strconv.FormatInt(int64(q.Days), 10)
	if v, ok := h.forecastCache.Get(key); ok {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.workerRPCTimeout)
	defer cancel()

	resp, err := h.grcp.GetForecast(ctx, &weatherpb.GetForecastRequest{
		City: q.City,
		Days: q.Days,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.InvalidArgument:
				w.WriteHeader(http.StatusBadRequest)
			case codes.NotFound:
				w.WriteHeader(http.StatusNotFound)
			case codes.DeadlineExceeded:
				w.WriteHeader(http.StatusGatewayTimeout)
			case codes.Canceled:
				w.WriteHeader(statusClientClosedRequest)
			default:
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	outDays := make([]DailyForecast, 0, len(resp.Days))
	for _, d := range resp.Days {
		outDays = append(outDays, DailyForecast{
			Date: d.Date,
			TMin: d.TMin,
			TMax: d.TMax,
		})
	}

	out := ForecastResponse{
		City: resp.City,
		Days: outDays,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
	h.forecastCache.Set(key, out)
}
