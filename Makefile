export GOPRIVATE=bitbucket.org/geniusrabbit/*

APP_TAGS    := "nats"
BENCH_PKG   ?= ./...
BENCH_TIME  ?= 3s
BENCH_COUNT ?= 1

.PHONY: all
all: lint cover

.PHONY: lint
lint: golint ## Run linters

.PHONY: golint
golint:
	# golint -set_exit_status ./...
	golangci-lint run -v ./...

.PHONY: fmt
fmt: ## Run formatting code
	@echo "Fix formatting"
	@gofmt -w ${GO_FMT_FLAGS} $$(go list -f "{{ .Dir }}" ./...); if [ "$${errors}" != "" ]; then echo "$${errors}"; fi

.PHONY: test
test: ## Run unit tests
	go test -v -tags ${APP_TAGS} -race ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: cover
cover:
	@mkdir -p $(TMP_ETC)
	@rm -f $(TMP_ETC)/coverage.txt $(TMP_ETC)/coverage.html
	go test -race -coverprofile=$(TMP_ETC)/coverage.txt -coverpkg=./... ./...
	@go tool cover -html=$(TMP_ETC)/coverage.txt -o $(TMP_ETC)/coverage.html
	@echo
	@go tool cover -func=$(TMP_ETC)/coverage.txt | grep total
	@echo
	@echo Open the coverage report:
	@echo open $(TMP_ETC)/coverage.html

.PHONY: bench
bench: ## Run benchmarks (BENCH_PKG=./..., BENCH_TIME=3s, BENCH_COUNT=1)
	go test -tags ${APP_TAGS} -run='^$$' -bench=. -benchmem \
		-benchtime=${BENCH_TIME} -count=${BENCH_COUNT} ${BENCH_PKG}

.PHONY: compile-rules
compile-rules: ## Compile JSON rule files from rules/data/ into Go sources
	@echo "Compile rules"
	@python3 scripts/compile-rules.py -in rules/data -out rules
	@gofmt -w ${GO_FMT_FLAGS} $$(go list -f "{{ .Dir }}" ./...); if [ "$${errors}" != "" ]; then echo "$${errors}"; fi

.PHONY: compile-sources
compile-sources: ## Compile JSON source-info files from sources/data/ into Go sources
	@echo "Compile sources"
	@python3 scripts/compile-sources.py -in sources/data -out sources
	@gofmt -w ${GO_FMT_FLAGS} $$(go list -f "{{ .Dir }}" ./...); if [ "$${errors}" != "" ]; then echo "$${errors}"; fi

.PHONY: generate-code
generate-code: ## Run codegeneration procedure
	@echo "Generate code"
	@go generate ./...

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
