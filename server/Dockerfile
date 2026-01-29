FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/portopener-server ./server/cmd/portopener-server

FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=build /out/portopener-server /app/portopener-server
COPY web /app/web
COPY migrations /app/migrations

ENV PORTOPENER_HTTP_ADDR=:8080
ENV PORTOPENER_WEB_ROOT=/app/web
ENV PORTOPENER_MIGRATIONS_DIR=/app/migrations

EXPOSE 8080
ENTRYPOINT ["/app/portopener-server"]

