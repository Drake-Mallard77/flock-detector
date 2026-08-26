FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/importer . \
 && CGO_ENABLED=0 go build -o /out/derive-deployments ./cmd/derive-deployments

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /out/importer /usr/local/bin/importer
COPY --from=build /out/derive-deployments /usr/local/bin/derive-deployments
# No default CMD: this image backs two different scheduled jobs, and the
# Cloud Run Job definition picks which binary runs. A default would make it
# ambiguous which one a bare `docker run` was doing.
