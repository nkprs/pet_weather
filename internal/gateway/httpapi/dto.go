package httpapi

type CurrentWeatherQuery struct {
	City string `validate:"required,min=2,max=64"`
}

type CurrentWeatherResponse struct {
	City         string  `json:"city"`
	TemperatureC float64 `json:"temperature_c"`
	WindSpeedMs  float64 `json:"wind_speed_ms"`
	WeatherCode  int32   `json:"weather_code"`
	ObservedAt   string  `json:"observed_at"`
}

type ForecastQuery struct {
	City string `validate:"required,min=2,max=64"`
	Days int32  `validate:"required,min=1,max=16"`
}

type ForecastResponse struct {
	City string          `json:"city"`
	Days []DailyForecast `json:"days"`
}

type DailyForecast struct {
	Date string  `json:"date"`
	TMin float64 `json:"t_min"`
	TMax float64 `json:"t_max"`
}
