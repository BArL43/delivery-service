FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/auth-service ./cmd/api

FROM alpine:3.20

RUN adduser -D appuser

COPY --from=builder /bin/auth-service /usr/local/bin/auth-service

USER appuser
EXPOSE 8081

CMD ["auth-service"]
