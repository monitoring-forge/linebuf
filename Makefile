VERSION=0.0.1
all: check lint bench

.PHONY: check lint bench

check: *.go
	go test -v ./...
	go test -race ./...

lint:
	golangci-lint run --timeout 5m ./...

bench: *.go
	go test -run='^$$' -bench=BenchmarkScan -benchmem -benchtime=5s ./...
