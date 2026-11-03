BINARY  := dnspc
PKG     := github.com/jeliasson/dns-propagation-checker
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
IMAGE   ?= dns-propagation-checker

GO_LDFLAGS := -s -w -X $(PKG)/internal/cli.Version=$(VERSION)

export CGO_ENABLED = 0

.PHONY: all build run test race cover lint fmt tidy docker docker-run clean

all: lint test build

build: ## Build the static binary into dist/
	go build -trimpath -ldflags "$(GO_LDFLAGS)" -o dist/$(BINARY) ./cmd/$(BINARY)
	@echo "binary: $$(du -h dist/$(BINARY) | cut -f1)"

run: ## Run the web app on :3000
	go run ./cmd/$(BINARY) serve

test:
	go test ./...

race: ## Race detector needs cgo, so it opts back in
	CGO_ENABLED=1 go test -race ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint:
	go vet ./...
	gofmt -l . | tee /dev/stderr | (! read)

fmt:
	gofmt -w .

tidy:
	go mod tidy

docker: ## Build the scratch image
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .
	@docker image ls --format '{{.Repository}}:{{.Tag}}  {{.Size}}' $(IMAGE):latest

docker-run: docker
	docker run --rm -p 3000:3000 $(IMAGE):latest

clean:
	rm -rf dist coverage.out
