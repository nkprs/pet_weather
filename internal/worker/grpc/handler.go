package grpc

import (
	"context"
	"errors"
	"strings"

	"weather/internal/worker/domain"
	"weather/internal/worker/service"
	weatherpb "weather/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Handler struct {
	weatherpb.UnimplementedWeatherServiceServer
	svc *service.WeatherService
}

func NewHandler(svc *service.WeatherService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) GetCurrentWeather(ctx context.Context, req *weatherpb.GetCurrentWeatherRequest) (*weatherpb.GetCurrentWeatherResponse, error) {
	city := domain.City(strings.TrimSpace(req.GetCity()))
	res, err := h.svc.Current(ctx, city)
	if err != nil {
		return nil, mapErr(err)
	}

	return &weatherpb.GetCurrentWeatherResponse{
		City:         res.City.String(),
		TemperatureC: res.TemperatureC,
		WindSpeedMs:  res.WindSpeedMs,
		WeatherCode:  res.WeatherCode,
		ObservedAt:   timestamppb.New(res.ObservedAt.UTC()),
	}, nil
}

func (h *Handler) GetForecast(ctx context.Context, req *weatherpb.GetForecastRequest) (*weatherpb.GetForecastResponse, error) {
	city := domain.City(strings.TrimSpace(req.GetCity()))
	res, err := h.svc.Forecast(ctx, city, req.GetDays())
	if err != nil {
		return nil, mapErr(err)
	}

	days := make([]*weatherpb.DailyForecast, 0, len(res.Days))
	for _, d := range res.Days {
		days = append(days, &weatherpb.DailyForecast{
			Date: d.Date.UTC().Format("2006-01-02"),
			TMin: d.TMin,
			TMax: d.TMax,
		})
	}

	return &weatherpb.GetForecastResponse{
		City: res.City.String(),
		Days: days,
	}, nil
}

func mapErr(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, err.Error())
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, err.Error())
	case errors.Is(err, domain.ErrInvalidCity), errors.Is(err, domain.ErrInvalidDays):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrCityNotFound):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
