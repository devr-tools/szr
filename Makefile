GOCACHE ?= $(CURDIR)/.gocache
GO ?= go
TESTPKG := ./test/...
COVERPKG := ./internal/...
COVERFILE := .coverage.internal.out
MIN_INTERNAL_COVERAGE ?= 80.0
GOVULNCHECK_MODE ?= warn
SMOKE_HOME := $(CURDIR)/.tmp-home
GOVULNCHECK_VERSION ?= v1.1.1
GOCYCLO_VERSION ?= v0.6.0
BASE_REF ?= $(shell git symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null | sed 's|^origin/||')
DOCKER ?= docker
CI_DOCKER_IMAGE ?= szr-ci:local
CI_DOCKER_GOCACHE ?= /workspace/.gocache-docker
CI_DOCKER_SMOKE_HOME ?= /tmp/szr-smoke-home

.PHONY: help fmt test cover cover-func cover-html build smoke ci ci-docker prepush clean

help:
	@printf '%s\n' \
		'make fmt        - gofmt the project files' \
		'make test       - run the centralized test suite in test/' \
		'make cover      - run tests with the internal coverage gate' \
		'make cover-func - print per-function coverage' \
		'make cover-html - render HTML coverage report' \
		'make build      - build ./bin/szr and ./bin/szr-dev' \
		'make smoke      - run quick local CLI smoke checks for szr and szr-dev, including bench/install flows' \
		'make ci         - run the host-mode local reproduction of the GitHub CI pipeline (override BASE_REF=...)' \
		'make ci-docker  - build and run the pinned Linux CI container with semgrep and govulncheck required' \
		'make ci         - run the local reproduction of the GitHub CI pipeline (override BASE_REF=...); also prints prerelease semver bump rules' \
		'make commit     - interactively git add ., choose commit type, commit, and push the current branch' \
		'make prepush    - run the quick local gate: fmt + test + cover + smoke' \
		'make clean      - remove local build and coverage artifacts'

fmt:
	env GOCACHE=$(GOCACHE) $(GO) fmt ./...

test:
	env GOCACHE=$(GOCACHE) $(GO) test $(TESTPKG)

cover:
	env GOCACHE=$(GOCACHE) $(GO) test $(TESTPKG) -coverpkg=$(COVERPKG) -coverprofile=$(COVERFILE)
	@total=$$(env GOCACHE=$(GOCACHE) $(GO) tool cover -func=$(COVERFILE) | awk '/^total:/ {print $$3}'); \
	if ! awk -v total="$$total" -v min="$(MIN_INTERNAL_COVERAGE)%" 'BEGIN { gsub(/%/, "", total); gsub(/%/, "", min); exit !(total + 0 >= min + 0) }'; then \
		echo "coverage gate failed: $$total (min $(MIN_INTERNAL_COVERAGE)%)"; \
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
	env HOME=$(SMOKE_HOME) GOCACHE=$(GOCACHE) $(GO) run ./cmd/szr bench clean-pass >/dev/null
	env HOME=$(SMOKE_HOME) GOCACHE=$(GOCACHE) $(GO) run ./cmd/szr install codex --print >/dev/null
	env HOME=$(SMOKE_HOME) GOCACHE=$(GOCACHE) $(GO) run ./cmd/szr-dev --version >/dev/null

ci:
	env \
		BASE_REF=$(BASE_REF) \
		GO=$(GO) \
		GOCACHE=$(GOCACHE) \
		MIN_INTERNAL_COVERAGE=$(MIN_INTERNAL_COVERAGE) \
		GOVULNCHECK_MODE=$(GOVULNCHECK_MODE) \
		GOVULNCHECK_VERSION=$(GOVULNCHECK_VERSION) \
		GOCYCLO_VERSION=$(GOCYCLO_VERSION) \
		SMOKE_HOME=$(SMOKE_HOME) \
		./scripts/ci.sh

ci-docker:
	$(DOCKER) build -f Dockerfile.ci -t $(CI_DOCKER_IMAGE) .
	$(DOCKER) run --rm -t \
		-v $(CURDIR):/workspace \
		-w /workspace \
		-e BASE_REF=$(BASE_REF) \
		-e GO=go \
		-e GOCACHE=$(CI_DOCKER_GOCACHE) \
		-e MIN_INTERNAL_COVERAGE=$(MIN_INTERNAL_COVERAGE) \
		-e GOVULNCHECK_MODE=required \
		-e GOVULNCHECK_VERSION=$(GOVULNCHECK_VERSION) \
		-e GOCYCLO_VERSION=$(GOCYCLO_VERSION) \
		-e SMOKE_HOME=$(CI_DOCKER_SMOKE_HOME) \
		$(CI_DOCKER_IMAGE) \
		./scripts/ci.sh
commit:
	@./scripts/commit.sh

prepush: fmt test cover smoke

clean:
	rm -rf ./bin ./coverage.html $(COVERFILE) $(SMOKE_HOME)
