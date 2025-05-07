# Latest as of 2025-05-04.
GOLANGCI_IMG = golangci/golangci-lint:v2.1.6-alpine@sha256:b122e5b85ddc99f62cb750039b5137247dda2327cbb96cac617bc0987be4f575
GOLANGCI_BIN = docker run --rm -it \
	-u "$(shell id -u):$(shell id -g)" \
	-v "${PWD}:/src" -w /src \
	-v $(shell go env GOMODCACHE || echo "${PWD}/.build/gomodcache"):/gomodcache \
	-e GOCACHE=/src/.build/gocache \
	-e GOMODCACHE=/gomodcache \
	-e GOLANGCI_LINT_CACHE=/src/.build/golangcicache \
	${GOLANGCI_IMG}

.PHONY: lint/go
lint/go:
	${GOLANGCI_BIN} golangci-lint run ./...
lint: lint/go
