default: all

PROJECT_NAME := "tick-framework"
PKG := "github.com/ltick/$(PROJECT_NAME)"
PKG_LIST := $(shell go list ./... | grep -v /vendor/)
GO_FILES := $(shell find . -name '*.go' | grep -v /vendor/ | grep -v _test.go)
REPORT_DIR := "test/reports"

default: test

test: ## Run unittests
	@go test -v ${PKG_LIST}
lint: ## Lint the files
	@golint ${PKG_LIST}
race: ## Run data race detector
	@go test -race -short ${PKG_LIST}
coverage: ## Generate global code coverage report
	./scripts/coverage.sh;
coverhtml: ## Generate global code coverage report in HTML
	./scripts/coverage.sh html ${REPORT_DIR};

.DEFAULT: test

.PHONY: test

