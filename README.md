# Pet Project: Weather Gateway (HTTP) + Weather Worker (gRPC)

## Цель проекта

Цель проекта — получить практические навыки backend-разработки на Go, реализовав минимальный, но архитектурно корректный сервис с:

- HTTP API (внешний слой)
- gRPC API (внутренний слой)
- интеграцией с внешним публичным API
- кешированием, таймаутами, контекстами
- чистой структурой проекта

Проект intentionally маленький, но приближённый к реальному продакшену.

## Общая архитектура

Проект состоит из двух сервисов:

1. Gateway Service (HTTP)
- Принимает HTTP-запросы от клиентов
- Валидирует входные параметры
- Кеширует ответы
- Вызывает Worker через gRPC
- Возвращает JSON-ответы клиенту

2. Worker Service (gRPC)
- Предоставляет gRPC API
- Ходит во внешний Weather API (Open-Meteo)
- Преобразует внешние данные в собственную доменную модель
- Возвращает нормализованные данные в Gateway

```text
Client
  ↓ HTTP
Gateway Service
  ↓ gRPC
Worker Service
  ↓ HTTP
Open-Meteo API
```

## Внешний API

Используется публичный API Open-Meteo:

- без API-ключей
- геокодинг по названию города
- текущая погода
- прогноз

Worker не должен проксировать внешний API напрямую: данные приводятся к собственному контракту.

## HTTP API (Gateway)

### `GET /healthz`

Проверка здоровья сервиса.

Ответ:

```json
{ "ok": true }
```

### `GET /v1/weather?city={city}`

Возвращает текущую погоду для города.

Пример ответа:

```json
{
  "city": "Helsinki",
  "temperature_c": -3.2,
  "wind_speed_ms": 4.1,
  "weather_code": 61,
  "observed_at": "2026-01-27T12:00:00Z"
}
```

Ошибки:

- `400` — отсутствует `city`
- `404` — город не найден
- `503` — worker недоступен
- `504` — таймаут

### `GET /v1/forecast?city={city}&days={N}`

Возвращает прогноз погоды на `N` дней (`1..16`).

Пример ответа:

```json
{
  "city": "Helsinki",
  "days": [
    { "date": "2026-01-28", "t_min": -5, "t_max": 1 },
    { "date": "2026-01-29", "t_min": -3, "t_max": 2 }
  ]
}
```

## gRPC API (Worker)

`WeatherService`:

- `GetCurrentWeather`
  - Вход: `city: string`
  - Выход: `city`, `temperature_c`, `wind_speed_ms`, `weather_code`, `observed_at`
- `GetForecast`
  - Вход: `city: string`, `days: int32`
  - Выход: `city`, `repeated DailyForecast`

## Кеширование (Gateway)

Кешировать ответы:

- `/weather` — 30-60 секунд
- `/forecast` — 2-5 минут

Реализация:

- `in-memory map + sync.RWMutex`
- опционально `singleflight` для дедупликации запросов

## Таймауты и контексты

- HTTP -> Gateway: учитывать `context.Context`
- Gateway -> Worker (gRPC): таймаут 1-2 секунды
- Worker -> Open-Meteo: таймаут 2 секунды

Все операции должны корректно реагировать на отмену контекста.

## Структура проекта (рекомендация)

```text
.
├── proto/
│   └── weather.proto
├── cmd/
│   ├── gateway/
│   │   └── main.go
│   └── worker/
│       └── main.go
├── internal/
│   ├── gateway/
│   │   ├── http/
│   │   ├── grpcclient/
│   │   └── cache/
│   └── worker/
│       ├── grpc/
│       ├── openmeteo/
│       └── service/
└── go.mod
```

## Технические требования

- Go >= 1.21
- HTTP: `net/http` (допустим `chi`)
- gRPC: `google.golang.org/grpc`
- Логирование: `log/slog`
- Конфигурация через `env`
- Без фреймворков типа Gin/Fiber

## Тестирование (минимум)

Worker:

- использовать `httptest.Server` для моков Open-Meteo

Gateway:

- использовать `bufconn` для моков gRPC worker

Проверить:

- happy-path
- ошибки
- таймауты

## Критерии готовности

- Gateway и Worker запускаются отдельно
- HTTP и gRPC контракты стабильны
- Нет прямого вызова внешнего API из Gateway
- Код читаемый, структурированный, без лишней магии
- Проект можно расширять без переписывания архитектуры

## Дополнительные улучшения (опционально)

- rate limiting в Gateway
- Prometheus метрики
- OpenTelemetry tracing
- Docker + docker-compose

## Local tracing with Docker

```bash
cd weather
docker compose up --build
```

После запуска:

- Gateway HTTP: `http://localhost:8081`
- Worker gRPC: `localhost:50051`
- Jaeger UI: `http://localhost:16686`

Smoke-запросы:

```bash
curl "http://localhost:8081/v1/weather?city=Helsinki"
curl "http://localhost:8081/v1/forecast?city=Helsinki&days=2"
```

В Jaeger UI ищите сервисы:

- `weather-gateway`
- `weather-worker`

## OpenTelemetry env

- `OTEL_ENABLED` (`true` по умолчанию)
- `OTEL_SERVICE_NAME` (если не задан, используется внутреннее имя сервиса)
- `OTEL_EXPORTER_OTLP_ENDPOINT` (`localhost:4317` по умолчанию)
- `OTEL_EXPORTER_OTLP_INSECURE` (`true` по умолчанию)

## Service env

Gateway:

- `GATEWAY_HTTP_ADDR` (`:8081` по умолчанию)
- `WORKER_GRPC_ADDR` (`127.0.0.1:50051` по умолчанию)
- `GATEWAY_WORKER_RPC_TIMEOUT` (`2s` по умолчанию)
- `GATEWAY_WEATHER_CACHE_TTL` (`30s` по умолчанию)
- `GATEWAY_FORECAST_CACHE_TTL` (`2m` по умолчанию)

Worker:

- `WORKER_GRPC_ADDR` (`:50051` по умолчанию)
- `WORKER_OPENMETEO_TIMEOUT` (`2s` по умолчанию)

## Proto generation

```bash
cd weather
docker compose --profile tools run --rm buf mod update
docker compose --profile tools run --rm buf generate
```
