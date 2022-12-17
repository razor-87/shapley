B=$(shell git rev-parse --abbrev-ref HEAD)
BRANCH=$(subst /,-,$(B))
GITREV=$(shell git describe --abbrev=7 --always --tags)
REV=$(GITREV)-$(BRANCH)
BENCH=go test -count=6 -benchmem -bench
GORUN=go run
GOBUILD=CGO_ENABLED=0 GOOS=linux go build

info:
	- @echo "revision $(REV)"

lint:
	@golangci-lint run

data:
	@python ./scripts/generate_data.py

test:
	go test -v ./...

test-race:
	go test -race -timeout=60s -count 1 ./...

run:
	@$(GORUN) . $(args)

run-race:
	@$(GORUN) -race . $(args)

bench-prepare:
	@$(BENCH)=BenchmarkPrepare -run=^$

bench-handle:
	@$(BENCH)=BenchmarkHandle -run=^$

bench-shapley:
	@$(BENCH)=BenchmarkShapley -run=^$

benchmarks: info
	@go test -bench=. -count=3 -benchmem -run=^$

profiles: info
	@$(GORUN) . -cpuprofile=true -genes=13 && $(GORUN) . -memprofile=true -genes=13

escape: info
	@$(GOBUILD) -v -gcflags "-m -m" && rm -rf ./shapley

build:
	@$(GOBUILD) -ldflags "-s -w"

.PHONY: info lint data test test-race run run-race bench-prepare bench-handle bench-shapley benchmarks profiles escape build
