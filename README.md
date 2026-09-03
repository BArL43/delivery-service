# Delivery Service

Учебный full-stack проект сервиса доставки, выполненный с фокусом на backend-разработку на Go. Репозиторий показывает работу с несколькими сервисами, PostgreSQL, Redis, JWT, Docker, CI/CD и observability.

## Что внутри

Система разделена на два Go-сервиса:

- `auth-service` отвечает за регистрацию, логин и выпуск JWT;
- `order-service` отвечает за заказы, курьеров, геокодирование, маршрутизацию и бизнес-метрики.

Входной трафик принимает Caddy. Данные хранятся в PostgreSQL, Redis используется как кеш геокодирования. Для маршрутов используется OSRM API.

```text
backend/
├── auth-service/
│   ├── cmd/api/
│   ├── internal/
│   └── migrations/
└── order-service/
    ├── cmd/api/
    ├── internal/
    └── migrations/

frontend/            React + Vite
observability/       Prometheus, Grafana, Loki, Promtail
Caddyfile             reverse proxy
Dockerfile.proxy      proxy image
docker-compose.yml    local stack
```

## Стек

**Backend:** Go, `net/http`, Gin, PostgreSQL, pgx, Redis, JWT  
**Infrastructure:** Docker, Docker Compose, Caddy  
**CI/CD:** GitHub Actions, GitLab CI/CD, Docker Hub  
**Observability:** Prometheus, Grafana, Loki, Promtail  
**Frontend:** React, Vite

## Что реализовано

- REST API для авторизации, заказов и курьеров;
- отдельные auth-service и order-service;
- PostgreSQL, SQL-миграции и транзакционные сценарии;
- JWT-аутентификация и проверка токена в order-service;
- Redis-кеш для геокодирования с fallback при недоступности кеша;
- интеграции с OpenStreetMap Nominatim и OSRM;
- graceful shutdown и HTTP timeouts;
- health checks;
- unit-тесты;
- multi-stage Docker-сборки с непривилегированным runtime-пользователем;
- CI для форматирования, `go vet`, race-тестов, сборки сервисов, frontend и Docker-образов;
- Prometheus-метрики, структурированные логи и Grafana dashboards.

## Целостность и безопасность backend

- JWT проверяется с фиксированным алгоритмом HS256 и issuer; секрет короче 32 байт отклоняется при старте сервиса;
- профиль курьера привязан к `user_id` из JWT, поэтому запрос не может выбрать другого курьера только подменой `courier_id`;
- назначение заказа выполняется одной PostgreSQL-транзакцией: создание assignment, закрепление заказа за курьером и смена статуса заказа либо применяются вместе, либо полностью откатываются;
- переходы статусов курьерского заказа ограничены допустимым графом, завершение или отмена заказа освобождает курьера в той же транзакции;
- координаты и основные входные данные валидируются до обращения к хранилищу.

## Локальный запуск

Требования: Docker и Docker Compose.

```bash
git clone https://github.com/BArL43/delivery-service.git
cd delivery-service
cp .env.example .env
docker compose up --build
```

`.env.example` содержит только локальные значения. Для любого публичного окружения нужно заменить `JWT_SECRET`, пароль PostgreSQL и доменные настройки.

После запуска приложение доступно через Caddy на `http://localhost`.

Остановка:

```bash
docker compose down
```

## Запуск сервисов отдельно

Auth service:

```bash
cd backend/auth-service
JWT_SECRET='local-development-secret-change-me-before-production' go run ./cmd/api
```

Order service:

```bash
cd backend/order-service
JWT_SECRET='local-development-secret-change-me-before-production' go run ./cmd/api
```

Для запуска вне Docker также нужны PostgreSQL и, для кеширования геокодирования, Redis.

## Проверки

```bash
cd backend/auth-service && go test ./... && go vet ./...
cd backend/order-service && go test ./... && go vet ./...
cd frontend && npm ci && npm run build
```

GitHub Actions дополнительно запускает race detector и проверяет сборку Docker-образов и `docker compose config`.

## Observability

Order service экспортирует Prometheus-метрики, включая количество HTTP-запросов, latency, число активных запросов и бизнес-события. Конфигурации Prometheus, Grafana, Loki и Promtail находятся в `observability/`.

Локально observability-стек можно поднять отдельно:

```bash
docker compose -f docker-compose.observability.yml up -d
```

## О проекте

Проект выполнен в рамках учебной практики по backend-разработке. Это не коммерческий production-сервис: репозиторий приведён в порядок как портфолио-проект с акцентом на Go, архитектуру backend-сервисов, работу с БД, инфраструктуру и качество кода.
