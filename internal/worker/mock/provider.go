package mock

import (
	"context"
	"time"
	"weather/internal/worker/domain"
)

type Provider struct{}

func NewProvider() *Provider { return &Provider{} }

func (p *Provider) Current(ctx context.Context, city domain.City) (domain.CurrentWeather, error) {
	now := time.Now().UTC()
	return domain.CurrentWeather{
		City:         city,
		TemperatureC: 5.5,
		WindSpeedMs:  3.2,
		WeatherCode:  61,
		ObservedAt:   now,
	}, nil
}

func (p *Provider) Forecast(ctx context.Context, city domain.City, days int32) (domain.Forecast, error) {
	out := make([]domain.DailyForecast, 0, days)
	start := time.Now().UTC()
	for i := int32(0); i < days; i++ {
		out = append(out, domain.DailyForecast{
			Date: start.AddDate(0, 0, int(i)),
			TMin: -3,
			TMax: 4,
		})
	}
	return domain.Forecast{City: city, Days: out}, nil
}
