package service

import (
	"context"
	"fmt"

	"weather/internal/worker/domain"
)

type WeatherService struct {
	provider domain.Provider
}

func NewWeatherService(p domain.Provider) *WeatherService {
	return &WeatherService{provider: p}
}

func (s *WeatherService) Current(ctx context.Context, city domain.City) (domain.CurrentWeather, error) {
	res, err := s.provider.Current(ctx, city)
	if err != nil {
		return domain.CurrentWeather{}, fmt.Errorf("current weather: %w", err)
	}
	return res, nil
}

func (s *WeatherService) Forecast(ctx context.Context, city domain.City, days int32) (domain.Forecast, error) {
	res, err := s.provider.Forecast(ctx, city, days)
	if err != nil {
		return domain.Forecast{}, fmt.Errorf("forecast: %w", err)
	}
	return res, nil
}
