FROM golang:1.22-alpine AS build

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/sledopyt-addresses ./cmd/sledopyt-addresses
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/gar77-fetch ./cmd/gar77-fetch

FROM alpine:3.20
WORKDIR /app
COPY --from=build /bin/sledopyt-addresses /usr/local/bin/sledopyt-addresses
COPY --from=build /bin/gar77-fetch /usr/local/bin/gar77-fetch

ENTRYPOINT ["/usr/local/bin/sledopyt-addresses"]
