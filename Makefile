.PHONY: build
build: tidy vet ## Build the binary
	go build -o build/kubernetes-events-exporter ./cmd/exporter

build-image: ## Build the Docker image
	docker build -t kubernetes-events-exporter .

.PHONY: fmt
fmt: ## Run go fmt against code
	gofmt -s -l -w .

.PHONY: vet
vet: ## Run go vet against code
	go vet ./...

tidy: ## Run go mod tidy
	go mod tidy

test: tidy ## Run tests
	go test -cover -mod=mod -v ./...

clean: ## Delete go.sum and clean mod cache
	go clean -modcache
	rm -f go.sum build/kubernetes-events-exporter

.PHONY: help
help: ## Display this help.
	@cat $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } '
