FROM golang:1.22-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/sledopyt-addresses ./cmd/sledopyt-addresses

FROM alpine:3.20
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=build /bin/sledopyt-addresses /usr/local/bin/sledopyt-addresses
USER appuser

ENTRYPOINT ["/usr/local/bin/sledopyt-addresses"]
