BINARY := tunneler
DIST_DIR := dist
CMD := ./cmd/tunneler
PKGS := ./...
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64

.PHONY: build build-all clean check fmt fmt-check lint test tidy tidy-check vet

build:
	mkdir -p $(DIST_DIR)
	go build -o $(DIST_DIR)/$(BINARY) $(CMD)

build-all:
	mkdir -p $(DIST_DIR)
	@set -e; for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output="$(DIST_DIR)/$(BINARY)-$${os}-$${arch}"; \
		if [ "$${os}" = "windows" ]; then output="$${output}.exe"; fi; \
		echo "building $${output}"; \
		GOOS=$${os} GOARCH=$${arch} go build -o "$${output}" $(CMD); \
	done

clean:
	rm -rf $(DIST_DIR)

check: tidy-check fmt-check vet lint test

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*'))" || \
		(gofmt -l $$(find . -name '*.go' -not -path './.git/*') && exit 1)

lint:
	@command -v golangci-lint >/dev/null 2>&1 || \
		(echo "golangci-lint is required; install the version used by CI" && exit 1)
	golangci-lint run

test:
	go test $(PKGS)

tidy:
	go mod tidy

tidy-check:
	go mod tidy
	git diff --exit-code -- go.mod go.sum

vet:
	go vet $(PKGS)
