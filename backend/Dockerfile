# Multi-stage build. Produces a ~20 MB image containing both the api and worker binaries.
# The entrypoint is overridden per ECS service (api vs worker vs migrate).

FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o /out/api    ./cmd/api    && \
    CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o /out/worker  ./cmd/worker && \
    CGO_ENABLED=0 GOOS=linux go build -mod=vendor -o /out/migrate ./cmd/migrate

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/ ./
# Default: run the API. ECS task definitions override this per service.
ENTRYPOINT ["/app/api"]
