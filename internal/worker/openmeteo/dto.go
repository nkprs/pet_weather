package openmeteo

type geocodeResp struct {
	Results []struct {
		Name      string  `json:"name"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"results"`
}

type forecastResp struct {
	Current struct {
		Time        string  `json:"time"`
		Temperature float64 `json:"temperature_2m"`
		WindSpeed   float64 `json:"wind_speed_10m"`
		WeatherCode int32   `json:"weather_code"`
	} `json:"current"`
	Daily struct {
		Time []string  `json:"time"`
		TMin []float64 `json:"temperature_2m_min"`
		TMax []float64 `json:"temperature_2m_max"`
	} `json:"daily"`
}
