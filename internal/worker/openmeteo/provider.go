package openmeteo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
	"weather/internal/worker/domain"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Provider struct {
	client  *http.Client
	baseURL string
}

func NewProvider(client *http.Client) *Provider {
	if client == nil {
		client = &http.Client{Timeout: 4 * time.Second}
	}

	baseTransport := client.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	client.Transport = otelhttp.NewTransport(baseTransport)

	return &Provider{client: client, baseURL: "https://api.open-meteo.com/v1"}
}

func (p *Provider) Current(ctx context.Context, city domain.City) (domain.CurrentWeather, error) {
	lat, lon, name, err := p.geocode(ctx, city.String())
	if err != nil {
		return domain.CurrentWeather{}, err
	}

	u, _ := url.Parse(p.baseURL + "/forecast")
	q := u.Query()
	q.Set("latitude", fmt.Sprintf("%f", lat))
	q.Set("longitude", fmt.Sprintf("%f", lon))
	q.Set("current", "temperature_2m,wind_speed_10m,weather_code")
	q.Set("wind_speed_unit", "ms")
	q.Set("temperature_unit", "celsius")
	q.Set("timezone", "UTC")
	u.RawQuery = q.Encode()

	var resp forecastResp
	if err := p.doJson(ctx, u.String(), &resp); err != nil {
		return domain.CurrentWeather{}, err
	}

	t, err := time.Parse(time.RFC3339, resp.Current.Time)
	if err != nil {
		// open-meteo иногда возвращает без "Z", но ISO8601 все равно парсится RFC3339
		t = time.Now().UTC()
	}

	return domain.CurrentWeather{
		City:         domain.City(name),
		TemperatureC: resp.Current.Temperature,
		WindSpeedMs:  resp.Current.WindSpeed,
		WeatherCode:  resp.Current.WeatherCode,
		ObservedAt:   t.UTC(),
	}, nil

}

func (p *Provider) Forecast(ctx context.Context, city domain.City, days int32) (domain.Forecast, error) {
	if days <= 0 {
		return domain.Forecast{}, domain.ErrInvalidDays
	}

	lat, lon, name, err := p.geocode(ctx, city.String())
	if err != nil {
		return domain.Forecast{}, err
	}

	u, _ := url.Parse(p.baseURL + "/forecast")
	q := u.Query()
	q.Set("latitude", fmt.Sprintf("%f", lat))
	q.Set("longitude", fmt.Sprintf("%f", lon))
	q.Set("daily", "temperature_2m_min,temperature_2m_max")
	q.Set("temperature_unit", "celsius")
	q.Set("timezone", "UTC")
	u.RawQuery = q.Encode()

	var resp forecastResp
	if err := p.doJson(ctx, u.String(), &resp); err != nil {
		return domain.Forecast{}, err
	}

	n := int(days)
	if n > len(resp.Daily.Time) {
		n = len(resp.Daily.Time)
	}

	out := make([]domain.DailyForecast, 0, n)
	for i := 0; i < n; i++ {
		d, err := time.Parse("2006-01-02", resp.Daily.Time[i])
		if err != nil {
			continue
		}
		out = append(out, domain.DailyForecast{
			Date: d.UTC(),
			TMin: resp.Daily.TMin[i],
			TMax: resp.Daily.TMax[i],
		})
	}

	return domain.Forecast{
		City: domain.City(name),
		Days: out,
	}, nil
}

func (p *Provider) geocode(ctx context.Context, city string) (lat, lon float64, name string, err error) {
	u, _ := url.Parse("https://geocoding-api.open-meteo.com/v1/search")
	q := u.Query()
	q.Set("name", city)
	q.Set("count", "1")
	q.Set("format", "json")
	u.RawQuery = q.Encode()

	var resp geocodeResp
	if err = p.doJson(ctx, u.String(), &resp); err != nil {
		return 0, 0, "", err
	}
	if len(resp.Results) == 0 {
		return 0, 0, "", domain.ErrCityNotFound
	}
	r := resp.Results[0]
	return r.Latitude, r.Longitude, r.Name, nil

}

func (p *Provider) doJson(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return errors.New(res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
}
