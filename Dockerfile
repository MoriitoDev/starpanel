# syntax=docker/dockerfile:1

# The Dashboard, built by Vite into the folder the Go package embeds. Node only
# exists in this stage: the runtime image has none.
FROM node:22-alpine AS web
WORKDIR /src
RUN corepack enable
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY apps/web/package.json apps/web/
RUN pnpm install --frozen-lockfile --filter @star-panel/web...
COPY apps/web apps/web
RUN pnpm --filter @star-panel/web build

# The binary, with that build inside it.
FROM golang:1.27-alpine AS api
WORKDIR /src
COPY apps/api/go.mod apps/api/
COPY apps/api/*.go apps/api/
COPY apps/api/internal apps/api/internal
COPY --from=web /src/apps/api/webdist apps/api/webdist
RUN go build -C apps/api -o /out/star-panel .

# What ships. The bundled Plugins are the two that need no runtime of their
# own: echo is left out because its backend is a Node script and this image has
# no Node. Copy it in and add Node if you want it.
FROM alpine:3.21
RUN apk add --no-cache ca-certificates \
 && adduser -D -u 10001 starpanel
WORKDIR /app
COPY --from=api /out/star-panel /app/star-panel
COPY apps/api/plugins/hello-widget /app/plugins/hello-widget
COPY apps/api/plugins/system-stats /app/plugins/system-stats
RUN mkdir -p /app/data /app/themes && chown -R starpanel:starpanel /app
USER starpanel
EXPOSE 8080
VOLUME ["/app/data", "/app/themes"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
  CMD wget -qO- http://127.0.0.1:8080/api/v1/health || exit 1
ENTRYPOINT ["/app/star-panel"]
