FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/flockwatch-api .

FROM alpine:3.24
RUN apk add --no-cache ca-certificates
COPY --from=build /out/flockwatch-api /usr/local/bin/flockwatch-api
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/flockwatch-api"]
