B=$(shell git rev-parse --abbrev-ref HEAD)
BRANCH=$(subst /,-,$(B))
GITREV=$(shell git describe --abbrev=7 --always --tags)
REV=$(GITREV)-$(BRANCH)

info:
	- @echo "revision $(REV)"

test:
	go test -v ./...

test-race:
	go test -race -timeout=60s -count 1 ./...

lint:
	@golangci-lint run

run:
	@go run -v .

run-race:
	@go run -race .

build:
	@go build -ldflags "-s -w"

build-linux:
	@CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w"

.PHONY: info test test-race lint run run-race build build-linux
