# Weather Gateway + Worker

## Local tracing with Docker

```bash
cd weather
docker compose up --build
```

After startup:
- Gateway HTTP: `http://localhost:8081`
- Worker gRPC: `localhost:50051`
- Jaeger UI: `http://localhost:16686`

Smoke request:

```bash
curl "http://localhost:8081/v1/weather?city=Helsinki"
curl "http://localhost:8081/v1/forecast?city=Helsinki&days=2"
```

Then open Jaeger UI and search services:
- `weather-gateway`
- `weather-worker`

## OpenTelemetry env

- `OTEL_ENABLED` (`true` by default)
- `OTEL_SERVICE_NAME` (defaults to internal fallback per service)
- `OTEL_EXPORTER_OTLP_ENDPOINT` (`localhost:4317` by default)
- `OTEL_EXPORTER_OTLP_INSECURE` (`true` by default)

## Proto generation

```bash
brew install buf
buf mod update
buf generate
```
