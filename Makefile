B=$(shell git rev-parse --abbrev-ref HEAD)
BRANCH=$(subst /,-,$(B))
GITREV=$(shell git describe --abbrev=7 --always --tags)
REV=$(GITREV)-$(BRANCH)
BENCH=go test -count=8 -benchmem -bench
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

gen:
	go generate gen.go

test:
	go test -v ./...

test-race:
	go test -race -timeout=60s -count 1 ./...

run:
	@$(GORUN) . $(args)

run-race:
	@$(GORUN) -race . $(args)

bench-prepare:
	@$(BENCH)=BenchmarkPrepare -benchtime=1000x -run=^$

bench-handle:
	@$(BENCH)=BenchmarkHandle -benchtime=2x -run=^$

bench-shapley:
	@$(BENCH)=BenchmarkShapley -benchtime=100x -run=^$

benchmarks:
	@go test -bench=. -count=4 -benchmem -run=^$

escape: info
	@$(GOBUILD) -v -gcflags "-m -m" && rm -rf ./shapley

cpu.prof:
	@$(GORUNMAX) -cpuprofile=true

mem.prof:
	@$(GORUNMAX) -memprofile=true

block.prof:
	@$(GORUNMAX) -blockprofile=true

pprof-cpu: info cpu.prof
	@$(PPROF) cpu.prof

pprof-mem: info mem.prof
	@$(PPROF) mem.prof

pprof-block: info block.prof
	@$(PPROF) block.prof

trace.out:
	@$(GORUNMAX) -trace=true

trace: info trace.out
	@go tool trace trace.out

benchstat:
	@sh ./scripts/benchstat.sh $(args)

build:
	@$(GOBUILD) -ldflags "-s -w"

.PHONY: info lint gen data test test-race run run-race bench-prepare bench-handle bench-shapley benchmarks escape pprof-cpu pprof-mem pprof-block trace benchstat build
