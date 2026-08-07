FROM golang:1.26.4-alpine AS build

WORKDIR /src

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/clinic-visits-api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build --chown=nonroot:nonroot /out/clinic-visits-api /app/clinic-visits-api

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/app/clinic-visits-api"]
