# syntax=docker/dockerfile:1

FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY server/ ./
COPY --from=web /web/dist ./cmd/gamedb/frontend
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /gamedb ./cmd/gamedb

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /gamedb /gamedb
USER nonroot
ENV DATA_DIR=/data
ENV HTTP_ADDR=:8080
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/gamedb"]
