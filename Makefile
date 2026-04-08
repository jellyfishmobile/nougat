# Stamp version / build number / time on every binary (use `make build`).
# Plain `go build` keeps dev defaults (0.0.0-dev / 0).

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")
BUILD_NUMBER ?= $(shell git rev-list --count HEAD 2>/dev/null || echo "0")
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X 'main.version=$(VERSION)' \
	-X 'main.buildNumber=$(BUILD_NUMBER)' \
	-X 'main.buildTime=$(BUILD_TIME)'

.PHONY: build
build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o nougat .

.PHONY: install
install:
	go install -trimpath -ldflags "$(LDFLAGS)" .

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test ./...
