# Delivery Service

Учебный full-stack проект сервиса доставки с двумя Go backend-сервисами, PostgreSQL, JWT-аутентификацией, контейнеризацией, CI/CD и observability-стеком.

> Основной фокус проекта — backend-разработка на Go и эксплуатационная часть: разделение сервисов, работа с БД, авторизация, тестирование, метрики и структурированные логи.

## Backend

Проект содержит два независимых Go-сервиса:

- **auth-service** — регистрация и логин пользователей, JWT-аутентификация;
- **order-service** — CRUD для заказов и курьеров, PostgreSQL и миграции.

Структура backend:

```text
backend/
├── auth-service/
│   ├── cmd/
│   ├── internal/
│   └── go.mod
└── order-service/
    ├── internal/
    ├── migrations/
    ├── main.go
    └── go.mod
```

## Tech stack

**Backend:** Go, REST API, PostgreSQL, JWT  
**Infrastructure:** Docker, Docker Compose, nginx  
**CI/CD:** GitLab CI/CD, Docker Hub, SSH deploy  
**Observability:** Prometheus, Grafana, Loki, Promtail  
**Frontend:** React, Vite

## Что реализовано

- отдельные auth-service и order-service;
- REST API для авторизации, заказов и курьеров;
- JWT-аутентификация;
- PostgreSQL и миграции;
- Docker-образы и Docker Compose;
- nginx как reverse proxy;
- CI/CD pipeline: test → build → Docker image → deploy;
- HTTP-метрики и business-метрики в Prometheus;
- JSON-логирование приложений;
- централизованные логи через Loki/Promtail;
- Grafana dashboards для инфраструктурных и бизнес-метрик.

## Observability

Prometheus собирает метрики обоих backend-сервисов. Среди них:

- `delivery_http_requests_total` — число HTTP-запросов;
- `delivery_http_request_duration_seconds` — latency;
- `delivery_http_inflight_requests` — активные запросы;
- `delivery_business_events_total` — бизнес-события с labels `event` и `result`.

HTTP-запросы и бизнес-события логируются в JSON. Loki хранит логи, а Grafana используется для анализа метрик и логов.

## CI/CD

`.gitlab-ci.yml` выполняет:

1. тестирование Go-сервисов;
2. сборку frontend;
3. сборку Docker-образов;
4. публикацию образов в Docker Hub;
5. обновление compose-стека на сервере по SSH.

## Локальный запуск

Самый простой способ — Docker Compose:

```bash
git clone https://github.com/BArL43/delivery-service.git
cd delivery-service
docker compose up --build
```

После запуска nginx принимает запросы на порту `80`.

Остановка:

```bash
docker compose down
```

### Запуск Go-сервисов отдельно

Order service:

```bash
cd backend/order-service
go mod tidy
go run .
```

Auth service:

```bash
cd backend/auth-service
go mod tidy
go run ./cmd/api
```

Пример конфигурации auth-service:

```env
AUTH_PORT=8081
AUTH_DB_DSN=postgres://user:pass@localhost:5432/dbname?sslmode=disable
JWT_SECRET=replace-me
```

## Мониторинг локально

```bash
docker compose -f docker-compose.observability.yml up -d
```

После запуска Grafana доступна на `http://localhost:3000`.

## Context

Проект выполнен в рамках учебной практики по backend-разработке. Репозиторий сохранён как портфолио-проект с акцентом на Go, инфраструктуру и эксплуатацию backend-сервисов.
