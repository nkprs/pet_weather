# Pet Project: Weather Gateway (HTTP) + Weather Worker (gRPC)

## Цель проекта

Цель проекта — получить практические навыки backend-разработки на **Go (Golang)**, реализовав минимальный, но архитектурно корректный сервис с:

- HTTP API (внешний слой)
- gRPC API (внутренний слой)
- интеграцией с внешним публичным API
- кешированием, таймаутами, контекстами
- чистой структурой проекта

Проект intentionally маленький, но приближённый к реальному продакшену.

---

## Общая архитектура

Проект состоит из **двух сервисов**:

### 1. Gateway Service (HTTP)
- Принимает HTTP-запросы от клиентов
- Валидирует входные параметры
- Кеширует ответы
- Вызывает Worker через gRPC
- Возвращает JSON-ответы клиенту

### 2. Worker Service (gRPC)
- Предоставляет gRPC API
- Ходит во внешний Weather API (Open-Meteo)
- Преобразует внешние данные в собственную доменную модель
- Возвращает нормализованные данные в Gateway

Client
↓ HTTP
Gateway Service
↓ gRPC
Worker Service
↓ HTTP
Open-Meteo API

yaml
Копировать код

---

## Внешний API

Использовать публичный API **Open-Meteo**:
- Без API-ключей
- Поддерживает:
  - Геокодинг по названию города
  - Текущую погоду
  - Прогноз

Worker **НЕ должен** проксировать внешний API напрямую — данные должны быть приведены к собственному контракту.

---

## HTTP API (Gateway)

### Endpoints

#### `GET /healthz`
Проверка здоровья сервиса.

Ответ:
```json
{ "ok": true }
GET /v1/weather?city={city}
Возвращает текущую погоду для города.

Пример ответа:

json
Копировать код
{
  "city": "Helsinki",
  "temperature_c": -3.2,
  "wind_speed_ms": 4.1,
  "weather_code": 61,
  "observed_at": "2026-01-27T12:00:00Z"
}
Ошибки:

400 — отсутствует city

404 — город не найден

503 — worker недоступен

504 — таймаут

GET /v1/forecast?city={city}&days={N}
Возвращает прогноз погоды на N дней.

Пример ответа:

json
Копировать код
{
  "city": "Helsinki",
  "days": [
    { "date": "2026-01-28", "t_min": -5, "t_max": 1 },
    { "date": "2026-01-29", "t_min": -3, "t_max": 2 }
  ]
}
gRPC API (Worker)
WeatherService
GetCurrentWeather
Вход:

city: string

Выход:

city

temperature_c

wind_speed_ms

weather_code

observed_at

GetForecast
Вход:

city: string

days: int32

Выход:

city

repeated DailyForecast

Кеширование (Gateway)
Кешировать ответы:

/weather — 30–60 секунд

/forecast — 2–5 минут

Реализация:

in-memory map + sync.RWMutex

(Опционально) singleflight для дедупликации запросов

Таймауты и контексты
HTTP → Gateway: учитывать context.Context

Gateway → Worker (gRPC): таймаут 1–2 секунды

Worker → Open-Meteo: таймаут 2 секунды

Все операции должны корректно реагировать на отмену контекста

Структура проекта (рекомендация)
go
Копировать код
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
├── go.mod
└── TASK.md
Технические требования
Go ≥ 1.21

HTTP: net/http (допустим chi)

gRPC: google.golang.org/grpc

Логирование: log/slog

Конфигурация через env

Без фреймворков типа Gin/Fiber

Тестирование (минимум)
Worker:

использовать httptest.Server для моков Open-Meteo

Gateway:

использовать bufconn для моков gRPC worker

Проверить:

happy-path

ошибки

таймауты

Критерии готовности
Gateway и Worker запускаются отдельно

HTTP и gRPC контракты стабильны

Нет прямого вызова внешнего API из Gateway

Код читаемый, структурированный, без лишней магии

Проект можно расширять без переписывания архитектуры

Дополнительные улучшения (опционально)
Rate limiting в Gateway

Prometheus метрики

OpenTelemetry tracing

Docker + docker-compose