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

## Комментарий по текущему состоянию

- order-service имеет рабочую точку входа в main.go.
- auth-service пока содержит пустую точку входа в cmd/api/main.go и требует донастройки инициализации БД/роутов.
