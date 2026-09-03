FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/auth-service ./cmd/api
RUN CGO_ENABLED=0 GOBIN=/out go install github.com/pressly/goose/v3/cmd/goose@v3.19.2

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app
WORKDIR /app
COPY --from=builder /out/auth-service /app/auth-service
COPY --from=builder /out/goose /usr/local/bin/goose
COPY --from=builder /src/migrations /app/migrations
USER app
EXPOSE 8081
CMD ["/app/auth-service"]
