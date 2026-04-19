# Микросервисная система мониторинга на Go

Программный прототип к выпускной квалификационной работе  
**«Разработка микросервисной архитектуры системы мониторинга на языке Go»**  
Ковалев Вячеслав Дмитриевич, РГСУ, направление 09.03.04 Программная инженерия

---

## Содержание

- [Архитектура](#архитектура)
- [Технологический стек](#технологический-стек)
- [Структура проекта](#структура-проекта)
- [Быстрый старт](#быстрый-старт)
- [API-интерфейс](#api-интерфейс)
- [Тестирование](#тестирование)
- [Развёртывание в Kubernetes](#развёртывание-в-kubernetes)
- [Наблюдаемость](#наблюдаемость)

---

## Архитектура

Система состоит из трёх независимых микросервисов, каждый из которых решает одну строго ограниченную задачу.

```
Внешние сервисы
      │
      │ HTTP POST /api/v1/{metrics,logs,traces}
      ▼
┌─────────────┐      NATS JetStream       ┌──────────────┐
│  collector  │ ─── telemetry.* ────────▶ │  aggregator  │
│  (порт 8080)│                           │              │
└─────────────┘                           └──────┬───────┘
                                                 │ INSERT
                                                 ▼
                                         ┌──────────────┐
                                         │  PostgreSQL   │
                                         └──────┬───────┘
                                                │ SELECT
                                                ▼
                                         ┌──────────────┐
                                         │     api      │
                                         │  (порт 8081) │
                                         └──────────────┘
```

**Поток данных:**
1. Внешний сервис отправляет телеметрию (метрики / логи / трассировки) в **collector** по HTTP.
2. Collector немедленно публикует сообщение в **NATS JetStream** (асинхронный буфер), освобождая входящий поток.
3. **Aggregator** подписывается на все субъекты `telemetry.*`, декодирует сообщения и записывает их в **PostgreSQL** через воспроизводимые миграции `golang-migrate`.
4. Внешние клиенты запрашивают накопленные данные через REST API сервиса **api**.

Каждый сервис:
- экспортирует Prometheus-метрики на `/metrics`;
- передаёт сквозной OpenTelemetry trace context;
- пишет структурированные JSON-логи через `zap`.

---

## Технологический стек

| Роль | Инструмент | Обоснование |
|------|-----------|-------------|
| Язык | **Go 1.23** | Компактные сетевые сервисы, удобная модель конкурентности, простой деплой |
| HTTP-фреймворк | **Gin** | Баланс между скоростью, удобством middleware и совместимостью с Go-экосистемой |
| Асинхронный транспорт | **NATS JetStream** | Буферизация потока телеметрии, гарантированная доставка, low-overhead |
| Внутренний RPC-контракт | **gRPC + Protobuf** | Строгая типизация, бинарная сериализация, streaming |
| Хранилище | **PostgreSQL 16** | Надёжное хранение временных рядов метрик, логов и трассировок |
| Миграции | **golang-migrate** | Воспроизводимое изменение схемы без ручных SQL-операций |
| Метрики | **Prometheus client** | Быстрый `/metrics` endpoint, интеграция с Kubernetes |
| Трассировка | **OpenTelemetry** | Vendor-neutral сквозной trace context через все сервисы |
| Логирование | **zap** | Структурированный JSON, пригодный для машинной корреляции |
| Контейнеризация | **Docker** | Воспроизводимая среда выполнения каждого сервиса |
| Оркестрация (локально) | **Docker Compose** | Единый контур запуска с зависимостями и healthcheck |
| Оркестрация (prod) | **Kubernetes** | Декларативное управление, HPA, rolling update |
| CI | **GitHub Actions** | Линт → тесты → интеграционные тесты → сборка образов |
| Линтинг | **golangci-lint** | Раннее выявление дефектов конкурентности и обработки ошибок |
| Тесты | **testify + Testcontainers** | Unit-тесты с fake-заглушками и интеграционные тесты с реальным PostgreSQL |

---

## Структура проекта

```
monitoring-system/
├── cmd/
│   ├── collector/          # Приём телеметрии по HTTP → NATS
│   │   ├── main.go
│   │   ├── server.go       # Gin-роутер + middleware
│   │   ├── handler.go      # HTTP-обработчики, интерфейс publisher
│   │   └── handler_test.go # Unit-тесты с fake NATS
│   ├── aggregator/         # NATS → PostgreSQL
│   │   ├── main.go
│   │   └── consumer.go     # Параллельные потребители по каждому типу сигнала
│   └── api/                # REST API для чтения данных
│       ├── main.go
│       ├── router.go       # Gin-роутер
│       └── handler.go      # Обработчики с OTel-трассировкой
├── internal/
│   ├── config/             # Конфигурация из переменных окружения
│   ├── logger/             # Инициализация zap
│   ├── metrics/            # Prometheus-регистры и HTTP-хендлер
│   ├── nats/               # JetStream: создание стрима, Publish, Subscribe
│   ├── otel/               # TracerProvider (stdout/OTLP), propagator
│   └── storage/
│       ├── postgres.go     # Пул соединений pgx + запуск миграций
│       ├── repository.go   # InsertMetric, QueryMetrics, InsertLog, InsertSpan …
│       └── repository_test.go  # Интеграционные тесты через Testcontainers
├── proto/telemetry/
│   └── telemetry.proto     # gRPC-контракт: MetricPoint, LogEntry, TraceSpan
├── migrations/
│   ├── 000001_init.up.sql  # Создание таблиц metrics, logs, spans + индексы
│   └── 000001_init.down.sql
├── deployments/
│   ├── docker-compose.yml  # Локальная среда: postgres, nats, все сервисы
│   └── k8s/                # Kubernetes манифесты
│       ├── namespace.yaml
│       ├── collector.yaml  # Deployment + Service + HPA
│       ├── aggregator.yaml
│       └── api.yaml
├── .github/workflows/
│   └── ci.yml              # lint → test → integration-test → build
├── .golangci.yml           # Конфигурация линтера
├── Dockerfile.collector
├── Dockerfile.aggregator
├── Dockerfile.api
├── Makefile
└── go.mod
```

---

## Быстрый старт

### Требования

- [Go 1.23+](https://go.dev/dl/)
- [Docker + Docker Compose](https://docs.docker.com/get-docker/)

### Локальный запуск

```bash
# 1. Клонировать / перейти в директорию
cd monitoring-system

# 2. Скачать зависимости
make tidy

# 3. Поднять инфраструктуру и сервисы
make docker-up
```

После запуска:
| Сервис | URL |
|--------|-----|
| Collector | http://localhost:8080 |
| API | http://localhost:8081 |
| NATS monitoring | http://localhost:8222 |

```bash
# Просмотр логов всех сервисов
make docker-logs

# Остановить и удалить тома
make docker-down
```

### Сборка без Docker

```bash
make build-collector
make build-aggregator
make build-api
```

### Генерация gRPC-кода из .proto

```bash
# Установить protoc-gen-go и protoc-gen-go-grpc:
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

make proto
```

---

## API-интерфейс

### Collector — приём телеметрии

#### POST /api/v1/metrics

```bash
curl -X POST http://localhost:8080/api/v1/metrics \
  -H "Content-Type: application/json" \
  -d '{
    "service_name": "payment-service",
    "metric_name":  "request_duration_seconds",
    "value":        0.123,
    "labels":       {"method": "POST", "status": "200"},
    "timestamp":    "2026-04-19T12:00:00Z"
  }'
```

#### POST /api/v1/logs

```bash
curl -X POST http://localhost:8080/api/v1/logs \
  -H "Content-Type: application/json" \
  -d '{
    "service_name": "auth-service",
    "level":        "error",
    "message":      "token validation failed",
    "trace_id":     "abc123def456",
    "fields":       {"user_id": "42", "ip": "10.0.0.1"}
  }'
```

#### POST /api/v1/traces

```bash
curl -X POST http://localhost:8080/api/v1/traces \
  -H "Content-Type: application/json" \
  -d '{
    "trace_id":       "abc123def456",
    "span_id":        "span-001",
    "service_name":   "order-service",
    "operation_name": "create_order",
    "start_time":     "2026-04-19T12:00:00Z",
    "end_time":       "2026-04-19T12:00:00.123Z",
    "status":         "ok",
    "attributes":     {"db.system": "postgresql"}
  }'
```

---

### API — чтение данных

#### GET /api/v1/metrics

```bash
curl "http://localhost:8081/api/v1/metrics?service=payment-service&from=2026-04-19T00:00:00Z&to=2026-04-19T23:59:59Z"
```

#### GET /api/v1/logs

```bash
curl "http://localhost:8081/api/v1/logs?service=auth-service&level=error&limit=50"
```

#### GET /api/v1/traces/:trace_id

```bash
curl "http://localhost:8081/api/v1/traces/abc123def456"
```

#### GET /health

```bash
curl http://localhost:8080/health
curl http://localhost:8081/health
```

---

## Тестирование

### Unit-тесты (без внешних зависимостей)

```bash
make test
# или напрямую:
go test -race -count=1 ./cmd/...
```

Тесты collector используют интерфейс `publisher` с fake-заглушкой вместо реального NATS — сервис тестируется изолированно.

### Интеграционные тесты (Testcontainers)

```bash
make integration-test
# или:
go test -race -count=1 -timeout=120s -tags=integration ./internal/storage/...
```

Поднимает реальный PostgreSQL-контейнер, применяет миграции и проверяет полный цикл записи и чтения для метрик, логов и трассировок.

### Линтинг

```bash
make lint
```

---

## Развёртывание в Kubernetes

```bash
# Создать namespace и применить манифесты
make k8s-apply

# Проверить статус
kubectl get pods -n monitoring
kubectl get hpa -n monitoring

# Удалить
make k8s-delete
```

Для каждого сервиса настроены:
- `Deployment` с readiness/liveness probe
- `HorizontalPodAutoscaler` (CPU > 70% → масштабирование)
- Prometheus-аннотации для автоматического scrape

Секрет с PostgreSQL URL создаётся отдельно:

```bash
kubectl create secret generic postgres-secret \
  --from-literal=url="postgres://user:pass@host:5432/db?sslmode=require" \
  -n monitoring
```

---

## Наблюдаемость

### Prometheus-метрики

Каждый сервис экспортирует на `/metrics`:
- `monitoring_<service>_requests_total{method, path, status}` — счётчик запросов
- `monitoring_<service>_request_duration_seconds{method, path}` — гистограмма задержек
- `monitoring_<service>_errors_total{type}` — счётчик ошибок
- стандартные Go runtime метрики (goroutines, GC, heap)

### OpenTelemetry-трассировка

По умолчанию трассировки выводятся в stdout (режим разработки).  
Для отправки в Jaeger / OpenTelemetry Collector установите переменную:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-collector:4317
```

### Структурированные логи

Все сервисы пишут JSON в stdout:

```json
{
  "ts": "2026-04-19T12:00:00.000Z",
  "level": "info",
  "service": "collector",
  "msg": "request",
  "method": "POST",
  "path": "/api/v1/metrics",
  "status": 202,
  "latency": "1.2ms"
}
```

Уровень логирования задаётся через `LOG_LEVEL` (debug / info / warn / error).

---

## Переменные окружения

| Переменная | Сервис | Значение по умолчанию |
|-----------|--------|----------------------|
| `HTTP_ADDR` | collector, api | `:8080` / `:8081` |
| `NATS_URL` | collector, aggregator | `nats://localhost:4222` |
| `POSTGRES_URL` | aggregator, api | `postgres://monitoring:monitoring@localhost:5432/monitoring?sslmode=disable` |
| `LOG_LEVEL` | все | `info` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | все | не задан (stdout) |
