COVERAGE_FILE ?= coverage.out

.PHONY: build
build: build_bot build_scrapper

.PHONY: build_bot
build_bot:
	@echo "Выполняется go build для таргета bot"
	@mkdir -p .bin
	@go build -o ./bin/bot ./cmd/bot

.PHONY: build_scrapper
build_scrapper:
	@echo "Выполняется go build для таргета scrapper"
	@mkdir -p .bin
	@go build -o ./bin/scrapper ./cmd/scrapper


## test: run all tests
.PHONY: test
test:
	@go test -coverpkg='github.com/central-university-dev/go-Matthew11K/...' --race -count=1 -coverprofile='$(COVERAGE_FILE)' ./...
	@go tool cover -func='$(COVERAGE_FILE)' | grep ^total | tr -s '\t'

.PHONY: lint
lint: lint-golang lint-proto

.PHONY: lint-golang
lint-golang:
	@if ! command -v 'golangci-lint' &> /dev/null; then \
  		echo "Please install golangci-lint!"; exit 1; \
  	fi;
	@golangci-lint -v run --fix ./...

.PHONY: lint-proto
lint-proto:
	@if ! command -v 'easyp' &> /dev/null; then \
  		echo "Please install easyp!"; exit 1; \
	fi;
	@easyp lint

.PHONY: generate
generate: generate_proto generate_openapi

.PHONY: generate_proto
generate_proto:
	@if ! command -v 'easyp' &> /dev/null; then \
		echo "Please install easyp!"; exit 1; \
	fi;
	@easyp generate

.PHONY: generate_openapi
generate_openapi:
	@if ! command -v 'ogen' &> /dev/null; then \
		echo "Please install ogen!"; exit 1; \
	fi;
	@mkdir -p internal/api/openapi/v1
	@ogen -package v1_bot \
		-target internal/api/openapi/v1_bot \
		api/openapi/v1/bot-api.yaml
	@ogen -package v1_scrapper \
		-target internal/api/openapi/v1_scrapper \
		api/openapi/v1/scrapper-api.yaml

.PHONY: clean
clean:
	@rm -rf./bin