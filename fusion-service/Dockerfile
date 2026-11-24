FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN go build -o /out/fusion-service ./cmd/fusion-service

FROM alpine:3.20

# Migrations are embedded into the binary at build time (see
# internal/migrations/migrations.go's go:embed) — nothing extra to copy.
COPY --from=build /out/fusion-service /usr/local/bin/fusion-service

EXPOSE 8002

CMD ["fusion-service"]
