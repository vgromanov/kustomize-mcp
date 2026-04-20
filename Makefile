# kustomize-mcp — build and test shortcuts
.PHONY: all build install clean test cover race fmt vet lint tidy dist help

BINARY_NAME := kustomize-mcp
CMD_PKG     := ./cmd/kustomize-mcp
DIST_DIR    := dist

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

all: build test

build:
	CGO_ENABLED=0 go build -o $(BINARY_NAME) $(CMD_PKG)

install:
	CGO_ENABLED=0 go install $(CMD_PKG)

clean:
	rm -f $(BINARY_NAME) coverage.out
	rm -rf $(DIST_DIR)

test:
	go test ./... -count=1

# Coverage is scoped to internal packages to avoid toolchain issues when
# merging profiles that include cmd/*/main with some Go releases.
cover:
	go test ./internal/... -count=1 -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out | tail -n 5

race:
	go test ./internal/... -count=1 -race

fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

lint:
	golangci-lint run ./...

dist:
	@mkdir -p $(DIST_DIR)
	$(foreach platform,$(PLATFORMS),\
		$(eval OS := $(word 1,$(subst /, ,$(platform))))\
		$(eval ARCH := $(word 2,$(subst /, ,$(platform))))\
		CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) \
			go build -o $(DIST_DIR)/$(BINARY_NAME)-$(OS)-$(ARCH) $(CMD_PKG) && \
	) true

help:
	@echo "Targets: all build install clean test cover race fmt vet lint tidy dist help"
