FROM golang:1.24.5-alpine AS builder
ENV GOTOOLCHAIN=auto GOPRIVATE=github.com/immxrtalbeast/*
RUN apk add --no-cache git ca-certificates build-base
WORKDIR /app

ARG GITHUB_TOKEN
RUN if [ -n "$GITHUB_TOKEN" ]; then \
      git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"; \
    fi

COPY go.mod go.sum ./
RUN go mod download -x
COPY . .
RUN go build -ldflags="-s -w" -o /app/main ./cmd/main.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /app/.env /app/
COPY --from=builder /app/config ./config

ENTRYPOINT ["/app/main"]
CMD ["--config=/app/config/dev.yaml"]
