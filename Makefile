GO ?= go
GOCACHE ?= $(PWD)/.gocache
GOMODCACHE ?= $(PWD)/.gocache/mod
CACHE_ENV = GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE)

.PHONY: all fmt test build plugin integ integ-log clean

all: test

fmt:
	$(GO)fmt ./...

test:
	$(CACHE_ENV) $(GO) test ./...

build:
	$(CACHE_ENV) $(GO) build ./...

plugin:
	$(CACHE_ENV) $(GO) build -o bin/logplugin ./cmd/logplugin

integ-log:
	$(CACHE_ENV) $(GO) run ./integ/log.go

integ: integ-log

clean:
	rm -rf $(GOCACHE) bin
