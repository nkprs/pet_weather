package domain

import (
	"context"
	"errors"
	"time"
)

type City string

func (c City) String() string { return string(c) }

type CurrentWeather struct {
	City         City
	TemperatureC float64
	WindSpeedMs  float64
	WeatherCode  int32
	ObservedAt   time.Time
}

type DailyForecast struct {
	Date time.Time
	TMin float64
	TMax float64
}

type Forecast struct {
	City City
	Days []DailyForecast
}

type Provider interface {
	Current(ctx context.Context, city City) (CurrentWeather, error)
	Forecast(ctx context.Context, city City, days int32) (Forecast, error)
}

var (
	ErrCityNotFound = errors.New("city not found")
	ErrInvalidCity  = errors.New("invalid city")
	ErrInvalidDays  = errors.New("invalid days")
)
