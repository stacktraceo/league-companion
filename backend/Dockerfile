# Многоступенчатая сборка: тулчейн Go нужен только на этапе компиляции,
# в рантайм-образ он не едет.
FROM golang:1.25-alpine AS build

WORKDIR /src

# Зависимости - отдельным слоем: он пересобирается только при изменении
# go.mod/go.sum, а не на каждую правку кода.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 даёт статический бинарь - рантайм-образу не нужен ни libc, ни Go.
# Миграции вшиты через //go:embed (internal/storage/migrate.go), поэтому *.sql
# отдельно в образ не кладём и CLI migrate внутри не требуется.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.21

# ca-certificates нужен для HTTPS-запросов к Riot API: без них клиент падает
# на проверке сертификата. wget для healthcheck'а уже есть в busybox.
RUN apk add --no-cache ca-certificates

# Процессу root не нужен.
RUN adduser -D -u 10001 app
USER app

COPY --from=build /out/server /usr/local/bin/server

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/server"]
