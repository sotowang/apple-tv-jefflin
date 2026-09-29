FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api && CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate && CGO_ENABLED=0 go build -o /out/healthcheck ./cmd/healthcheck && CGO_ENABLED=0 go build -o /out/catalog-sync ./cmd/catalog-sync
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/migrate /app/migrate
COPY --from=build /out/healthcheck /app/healthcheck
COPY --from=build /out/catalog-sync /app/catalog-sync
COPY --from=build /src/db/migrations /app/migrations
EXPOSE 8080
CMD ["/app/api"]
