package openmeteo

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"weather/internal/worker/domain"
)

type rewriteTransport struct {
	base *url.URL
	rt   http.RoundTripper
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = t.base.Scheme
	clone.URL.Host = t.base.Host
	return t.rt.RoundTrip(clone)
}

func newTestProvider(t *testing.T, handler http.Handler, clientTimeout time.Duration) *Provider {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	base, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	client := &http.Client{
		Timeout: clientTimeout,
		Transport: &rewriteTransport{
			base: base,
			rt:   http.DefaultTransport,
		},
	}

	p := NewProvider(client)
	p.baseURL = srv.URL + "/v1"
	return p
}

func TestProviderCurrent_OK(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		resp := geocodeResp{Results: []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		}{
			{Name: "Helsinki", Latitude: 60.17, Longitude: 24.94},
		}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/v1/forecast", func(w http.ResponseWriter, r *http.Request) {
		resp := forecastResp{}
		resp.Current.Time = "2026-01-27T12:00:00Z"
		resp.Current.Temperature = -3.2
		resp.Current.WindSpeed = 4.1
		resp.Current.WeatherCode = 61
		_ = json.NewEncoder(w).Encode(resp)
	})

	p := newTestProvider(t, mux, 2*time.Second)

	res, err := p.Current(context.Background(), domain.City("Helsinki"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantTime := time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC)
	if res.City.String() != "Helsinki" {
		t.Fatalf("city mismatch: %s", res.City.String())
	}
	if res.TemperatureC != -3.2 || res.WindSpeedMs != 4.1 || res.WeatherCode != 61 {
		t.Fatalf("values mismatch: %+v", res)
	}
	if !res.ObservedAt.Equal(wantTime) {
		t.Fatalf("observed_at mismatch: %s", res.ObservedAt)
	}
}

func TestProviderForecast_OK(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		resp := geocodeResp{Results: []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		}{
			{Name: "Helsinki", Latitude: 60.17, Longitude: 24.94},
		}}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/v1/forecast", func(w http.ResponseWriter, r *http.Request) {
		resp := forecastResp{}
		resp.Daily.Time = []string{"2026-01-28", "2026-01-29", "2026-01-30"}
		resp.Daily.TMin = []float64{-5, -3, -2}
		resp.Daily.TMax = []float64{1, 2, 3}
		_ = json.NewEncoder(w).Encode(resp)
	})

	p := newTestProvider(t, mux, 2*time.Second)

	res, err := p.Forecast(context.Background(), domain.City("Helsinki"), 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.City.String() != "Helsinki" {
		t.Fatalf("city mismatch: %s", res.City.String())
	}
	if len(res.Days) != 2 {
		t.Fatalf("expected 2 days, got %d", len(res.Days))
	}
	if res.Days[0].TMin != -5 || res.Days[0].TMax != 1 {
		t.Fatalf("day0 mismatch: %+v", res.Days[0])
	}
}

func TestProviderCurrent_CityNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(geocodeResp{Results: nil})
	})

	p := newTestProvider(t, mux, 2*time.Second)

	_, err := p.Current(context.Background(), domain.City("Nowhere"))
	if !errors.Is(err, domain.ErrCityNotFound) {
		t.Fatalf("expected ErrCityNotFound, got %v", err)
	}
}

func TestProviderForecast_InvalidDays(t *testing.T) {
	mux := http.NewServeMux()
	p := newTestProvider(t, mux, 2*time.Second)

	_, err := p.Forecast(context.Background(), domain.City("Helsinki"), 0)
	if !errors.Is(err, domain.ErrInvalidDays) {
		t.Fatalf("expected ErrInvalidDays, got %v", err)
	}
}

func TestProviderCurrent_Timeout(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(geocodeResp{Results: nil})
	})

	p := newTestProvider(t, mux, 2*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := p.Current(ctx, domain.City("Helsinki"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
