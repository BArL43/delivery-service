# Delivery Service

Проект приведен к явному разделению на фронтенд и бэкенд.

## Структура

```text
delivery-service/
	frontend/
		package.json
		src/
		...
	backend/
		auth-service/
			go.mod
			cmd/
			internal/
		order-service/
			go.mod
			main.go
			internal/
			migrations/
	README.md
```

## Что где находится

- frontend: Vite + React клиентская часть.
- backend/auth-service: сервис авторизации (регистрация/логин, JWT).
- backend/order-service: сервис заказов (CRUD по orders, PostgreSQL).

## Запуск

### Frontend

```bash
cd frontend
npm install
npm run dev
```

### Backend: order-service

```bash
cd backend/order-service
go mod tidy
go run .
```

### Backend: auth-service

```bash
cd backend/auth-service
go mod tidy
go run ./cmd/api
```

По умолчанию auth-service запускается на порту 8081 и ожидает PostgreSQL по DSN:

```text
postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
```

Можно переопределить через переменные окружения:

```bash
AUTH_PORT=8081
AUTH_DB_DSN=postgres://user:pass@localhost:5432/dbname?sslmode=disable
JWT_SECRET=your-secret-key
```

Во frontend включен Vite proxy с маршрутом /api -> http://localhost:8081.

## Запуск через Docker Compose + nginx

В репозиторий добавлены:
тест
- docker-compose.yml
- nginx.conf

Сценарий:

- nginx принимает запросы на порту 80.
- `/api/*` и `/health` проксируются в auth-service.
- Все остальные запросы идут во frontend.

Команды запуска:

```bash
docker compose up --build
```

После старта приложение доступно по адресу:

```text
http://localhost
```

Остановка:

```bash
docker compose down
```

## Деплой через Docker Hub

Для сервера можно не собирать образы на месте, а тянуть готовые из Docker Hub.

Подготовка локально:

```bash
docker login
docker build -t <dockerhub_username>/delivery-auth-service:latest ./backend/auth-service
docker build -t <dockerhub_username>/delivery-frontend:latest ./frontend
docker push <dockerhub_username>/delivery-auth-service:latest
docker push <dockerhub_username>/delivery-frontend:latest
```

Подготовка на сервере:

```bash
cp .env.dockerhub.example .env.dockerhub
```

Отредактируй `.env.dockerhub`:

```text
DOCKERHUB_USERNAME=<dockerhub_username>
IMAGE_TAG=latest
```

Запуск на сервере из Docker Hub:

```bash
docker compose --env-file .env.dockerhub -f docker-compose.hub.yml up -d
```

Проверка:

```bash
docker compose --env-file .env.dockerhub -f docker-compose.hub.yml ps
```

## CI/CD

В репозитории есть конфиг в `.gitlab-ci.yml`.

Он делает следующее:

- тестирует `backend/auth-service` и `backend/order-service`;
- собирает и проверяет фронтенд;
- билдит и пушит Docker-образы в Docker Hub;
- подключается по SSH к серверу и обновляет compose-стек.

## Observability

В проект добавлен стек для локального мониторинга и логирования:

- Prometheus собирает метрики из `auth-service` и `order-service`.
- Loki хранит JSON-логи приложений.
- Promtail читает Docker logs и отправляет их в Loki.
- Grafana автоматически подхватывает datasources и дашборды.

### Что нужно сделать

1. Сначала задеплой приложение через CI/CD или вручную обычным compose.
2. Убедись, что приложение и observability-стек находятся в одной Docker network. По умолчанию это `delivery-service_default`.
3. Подними observability-стек:

```bash
docker compose -f docker-compose.observability.yml up -d
```

4. Открой Grafana на `http://<server>:3000`.
5. Логин по умолчанию: `admin / admin`.

### Доступные дашборды

- `Delivery Infra` - метрики HTTP, задержки, ошибки и активные запросы.
- `Delivery Business` - регистрация, логин, создание заказов и назначение курьеров.

### Основные метрики

- `delivery_http_requests_total`
- `delivery_http_request_duration_seconds`
- `delivery_http_inflight_requests`
- `delivery_business_events_total`

### Основные запросы Loki

- Все ошибки сервисов: `{service=~"auth-service|order-service"} |= "error"`
- Бизнес-события: `{service=~"auth-service|order-service"} | json | event != ""`

### Настройка в GitLab

В Settings → CI/CD → Variables добавить:

- `DOCKERHUB_USERNAME` — логин Docker Hub
- `CI_REGISTRY_USER` — логин Docker Hub (для `docker login`)
- `CI_REGISTRY_PASSWORD` — access token Docker Hub (не пароль)
- `SSH_HOST` — IP или домен сервера
- `SSH_USER` — пользователь на сервере (обычно `root`)
- `SSH_PORT` — порт SSH (обычно `22`)
- `SSH_PRIVATE_KEY` — приватный SSH-ключ целиком

### Требования на сервере

- Проект в `/opt/delivery-service`
- Файл `.env.dockerhub` с переменными:
  ```
  DOCKERHUB_USERNAME=<username>
  IMAGE_TAG=latest
  ```
- Docker и Docker Compose установлены

## Комментарий по текущему состоянию

- order-service имеет рабочую точку входа в main.go.
- auth-service поднимает API регистрации/логина в cmd/api/main.go.
