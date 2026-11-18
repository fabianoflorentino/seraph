# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.27.4-alpine AS build

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/seraph ./cmd/seraph

FROM --platform=$BUILDPLATFORM hugomods/hugo:0.158.0 AS site

ARG HUGO_BASEURL=https://seraph.srectl.fabianoflorentino.dev/
WORKDIR /src
COPY site ./site
RUN hugo --source site --minify --gc --cleanDestinationDir --baseURL "${HUGO_BASEURL}"

FROM nginxinc/nginx-unprivileged:1.27-alpine

USER root

COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /out/seraph /usr/local/bin/seraph
COPY --from=site /src/site/public /usr/share/nginx/html
COPY catalog /app/catalog
COPY config /app/config
COPY docker/entrypoint.sh /usr/local/bin/seraph-entrypoint

RUN mkdir -p /usr/share/nginx/html/data \
    && chmod +x /usr/local/bin/seraph-entrypoint \
    && chown -R 101:101 /app /usr/share/nginx/html /var/cache/nginx

USER 101:101

WORKDIR /app

ENV SERAPH_OUT_DIR=/usr/share/nginx/html/data \
    SERAPH_BASE_URL=https://seraph.srectl.fabianoflorentino.dev/data/state.json \
    SERAPH_SYNC_INTERVAL=300

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/seraph-entrypoint"]
