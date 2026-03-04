GO ?= go

.PHONY: test race vet fmt lint quality-gate

test:
	$(GO) test ./...

race:
	$(GO) test -race ./internal/...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './vendor/*')
	goimports -w $(shell find . -name '*.go' -not -path './vendor/*')

lint:
	golangci-lint run

quality-gate:
	bash ./scripts/quality-gate.sh
