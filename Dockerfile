# Both binaries, the migration tool, and the dashboard come from one image.
# The entrypoint selects api, worker, or migrate.
FROM golang:1.22-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.1

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/api ./cmd/api \
  && CGO_ENABLED=0 GOOS=linux go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/worker ./cmd/worker

FROM node:22-alpine AS dashboard

WORKDIR /src/dashboard
COPY dashboard/package.json dashboard/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile

COPY dashboard/ ./
RUN pnpm run build

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
 && adduser -D -u 10001 orcta

COPY --from=build /out/api /usr/local/bin/api
COPY --from=build /out/worker /usr/local/bin/worker
COPY --from=build /go/bin/migrate /usr/local/bin/migrate

COPY --from=build /src/migrations /migrations
COPY --from=dashboard /src/dashboard/dist /usr/local/share/dashboard

USER orcta
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/api"]
