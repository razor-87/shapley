B=$(shell git rev-parse --abbrev-ref HEAD)
BRANCH=$(subst /,-,$(B))
GITREV=$(shell git describe --abbrev=7 --always --tags)
REV=$(GITREV)-$(BRANCH)
BENCH=go test -count=6 -benchmem -bench
GORUN=go run
GORUNMAX=$(GORUN) . -genes=13
GOBUILD=CGO_ENABLED=0 GOOS=linux go build
PPROF=go tool pprof -http=:8000

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

escape: info
	@$(GOBUILD) -v -gcflags "-m -m" && rm -rf ./shapley

cpu.prof:
	@$(GORUNMAX) -cpuprofile=true

mem.prof:
	@$(GORUNMAX) -memprofile=true

pprof-cpu: info cpu.prof
	@$(PPROF) cpu.prof

pprof-mem: info mem.prof
	@$(PPROF) mem.prof

build:
	@$(GOBUILD) -ldflags "-s -w"

.PHONY: info lint data test test-race run run-race bench-prepare bench-handle bench-shapley benchmarks escape pprof-cpu pprof-mem build
