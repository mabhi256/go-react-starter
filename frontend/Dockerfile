# Frontend build for local docker compose. In prod, AWS Amplify builds from source.
FROM oven/bun:1-alpine AS builder
WORKDIR /app
COPY package.json bun.lock ./
RUN --mount=type=cache,target=/bun-cache \
    BUN_INSTALL_CACHE_DIR=/bun-cache bun install --frozen-lockfile
COPY . .
ARG VITE_API_URL=http://localhost:8080
RUN VITE_API_URL=$VITE_API_URL bun run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 3000
