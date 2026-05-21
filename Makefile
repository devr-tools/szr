GOCACHE ?= $(CURDIR)/.gocache
GO ?= go
TESTPKG := ./test/...
COVERPKG := ./internal/...
COVERFILE := .coverage.internal.out
SMOKE_HOME := $(CURDIR)/.tmp-home

.PHONY: help fmt test cover cover-func cover-html build smoke prepush clean

help:
	@printf '%s\n' \
		'make fmt        - gofmt the project files' \
		'make test       - run the centralized test suite in test/' \
		'make cover      - run tests with 100%% internal coverage enforcement' \
		'make cover-func - print per-function coverage' \
		'make cover-html - render HTML coverage report' \
		'make build      - build ./bin/szr and ./bin/szr-dev' \
		'make smoke      - run quick local CLI smoke checks for szr and szr-dev' \
		'make prepush    - fmt + test + cover + smoke' \
		'make clean      - remove local build and coverage artifacts'

fmt:
	env GOCACHE=$(GOCACHE) $(GO) fmt ./...

test:
	env GOCACHE=$(GOCACHE) $(GO) test $(TESTPKG)

cover:
	env GOCACHE=$(GOCACHE) $(GO) test $(TESTPKG) -coverpkg=$(COVERPKG) -coverprofile=$(COVERFILE)
	@total=$$(env GOCACHE=$(GOCACHE) $(GO) tool cover -func=$(COVERFILE) | awk '/^total:/ {print $$3}'); \
	if [ "$$total" != "100.0%" ]; then \
		echo "coverage gate failed: $$total"; \
		exit 1; \
	fi

cover-func:
	env GOCACHE=$(GOCACHE) $(GO) tool cover -func=$(COVERFILE)

cover-html:
	env GOCACHE=$(GOCACHE) $(GO) tool cover -html=$(COVERFILE) -o coverage.html

build:
	mkdir -p bin
	env GOCACHE=$(GOCACHE) $(GO) build -o ./bin/szr ./cmd/szr
	env GOCACHE=$(GOCACHE) $(GO) build -o ./bin/szr-dev ./cmd/szr-dev

smoke:
	mkdir -p $(SMOKE_HOME)
	env HOME=$(SMOKE_HOME) GOCACHE=$(GOCACHE) $(GO) run ./cmd/szr --help >/dev/null
	env HOME=$(SMOKE_HOME) GOCACHE=$(GOCACHE) $(GO) run ./cmd/szr profiles >/dev/null
	env HOME=$(SMOKE_HOME) GOCACHE=$(GOCACHE) $(GO) run ./cmd/szr explain git status >/dev/null
	env HOME=$(SMOKE_HOME) GOCACHE=$(GOCACHE) $(GO) run ./cmd/szr-dev --version >/dev/null

prepush: fmt test cover smoke

clean:
	rm -rf ./bin ./coverage.html $(COVERFILE) $(SMOKE_HOME)
