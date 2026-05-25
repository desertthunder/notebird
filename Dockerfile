# syntax=docker/dockerfile:1

FROM node:24-bookworm-slim AS frontend
WORKDIR /src

COPY frontend/package.json frontend/pnpm-lock.yaml ./frontend/
RUN corepack enable && pnpm --dir frontend install --frozen-lockfile

COPY frontend ./frontend
COPY internal/templates/html/static ./internal/templates/html/static
RUN pnpm --dir frontend run build

FROM golang:1.26-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend /src/internal/templates/html/static/dist ./internal/templates/html/static/dist
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/notebird ./cmd/notebird

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/notebird /notebird

EXPOSE 7331
VOLUME ["/data"]

ENTRYPOINT ["/notebird"]
CMD ["serve", "--host", "0.0.0.0", "--data-dir", "/data"]
