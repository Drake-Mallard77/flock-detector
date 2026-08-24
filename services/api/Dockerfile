FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/flockwatch-api .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/flockwatch-api /usr/local/bin/flockwatch-api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/flockwatch-api"]
