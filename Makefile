BINARY_NAME=sledopyt_addresses
.DEFAULT_GOAL := run

build:
	mkdir -p logs
	mkdir -p data
	go build -o ./${BINARY_NAME}-app ./cmd/main.go


run: build
	./${BINARY_NAME}-app -envtype testing


build_and_run: build run

clean:
	go clean
	rm ./${BINARY_NAME}-app


test:
	go test ./...


test_coverage:
	go test ./... -coverprofile=coverage.out


dep:
	go mod download

vet:
	go vet

lint:
	golangcli-lint run --enable-all
