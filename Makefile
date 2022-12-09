B=$(shell git rev-parse --abbrev-ref HEAD)
BRANCH=$(subst /,-,$(B))
GITREV=$(shell git describe --abbrev=7 --always --tags)
REV=$(GITREV)-$(BRANCH)
BENCH=go test -count=5 -benchmem -bench
GORUN=go run

info:
	- @echo "revision $(REV)"

data:
	@python ./scripts/generate_data.py

test:
	go test -v ./...

test-race:
	go test -race -timeout=60s -count 1 ./...

lint:
	@golangci-lint run

run:
	@$(GORUN) . $(args)

run-race:
	@$(GORUN) -race . $(args)

bench-prepare:
	@$(BENCH)=BenchmarkPrepare

bench-handle:
	@$(BENCH)=BenchmarkHandle

bench-shapley:
	@$(BENCH)=BenchmarkShapley

benchmarks: info
	@go test -bench=. -count=2 -benchmem

profiles: info
	@$(GORUN) . -cpuprofile=true && $(GORUN) . -memprofile=true

build:
	@go build -ldflags "-s -w" $(args)

build-linux:
	@CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" $(args)

.PHONY: info data test test-race lint run run-race bench-prepare bench-handle bench-shapley benchmarks profiles build build-linux
