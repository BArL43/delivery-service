FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/order-service ./cmd/api

FROM alpine:3.20

RUN adduser -D appuser

COPY --from-builder /bin/order-service /usr/local/bin/order-service

USER appuser
EXPOSE 8080

CMD ["order-service"]
