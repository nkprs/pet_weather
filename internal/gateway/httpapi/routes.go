package httpapi

import (
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("/healthz", h.GetHealthz)

	mux.HandleFunc("/v1/weather", h.GetCurrentWeather)
	mux.HandleFunc("/v1/forecast", h.GetForecast)
}
