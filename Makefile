BINARY=node-agent
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION)"

GO?=go
GOFMT=gofmt -s -w

PLATFORMS=linux/amd64 linux/arm64 linux/armv7

.PHONY: all build build-linux build-darwin clean test vet fmt release package

all: vet test build

build:
	$(GO) build $(LDFLAGS) -o $(BINARY) .

build-linux:
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY)-linux-amd64 .

build-darwin:
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BINARY)-darwin-arm64 .

test:
	$(GO) test ./... -v -count=1 -cover

test-short:
	$(GO) test ./... -count=1 -cover -short

vet:
	$(GO) vet ./...

fmt:
	$(GOFMT) .

clean:
	rm -f $(BINARY) $(BINARY)-*
	rm -rf dist/

release: clean
	@echo "Building release $(VERSION)..."
	@mkdir -p dist
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*}; \
		GOARCH=$${platform#*/}; \
		ARCH_SUFFIX="$$GOOS-$$GOARCH"; \
		if [ "$$GOARCH" = "armv7" ]; then \
			GOARCH=arm GOARM=7; \
			ARCH_SUFFIX="linux-armv7"; \
		fi; \
		echo "  Building $$ARCH_SUFFIX..."; \
		GOOS=$$GOOS GOARCH=$$GOARCH GOARM=$${GOARM:-} $(GO) build $(LDFLAGS) -o dist/$(BINARY) . || exit 1; \
		cp deploy/install.sh deploy/node-agent.service dist/; \
		tar -czf dist/$(BINARY)-$$ARCH_SUFFIX.tar.gz -C dist $(BINARY) install.sh node-agent.service; \
		rm -f dist/$(BINARY) dist/install.sh dist/node-agent.service; \
	done
	@echo ""
	@echo "Release packages:"
	@ls -lh dist/
	@echo ""
	@echo "SHA256:"
	@cd dist && sha256sum *.tar.gz || shasum -a 256 *.tar.gz

package: build
	@mkdir -p dist
	cp $(BINARY) dist/
	cp deploy/install.sh dist/
	cp deploy/node-agent.service dist/
	tar -czf dist/$(BINARY)-$(shell uname -s | tr '[:upper:]' '[:lower:]')-$(shell uname -m).tar.gz -C dist $(BINARY) install.sh node-agent.service
	@rm -f dist/$(BINARY) dist/install.sh dist/node-agent.service
	@echo "Package created: dist/$(BINARY)-$(shell uname -s | tr '[:upper:]' '[:lower:]')-$(shell uname -m).tar.gz"

install: build
	@echo "Installing via deploy script..."
	sudo bash deploy/install.sh

run: build
	./$(BINARY) -config /etc/node-agent/config.json

dev:
	$(GO) run . -config config.json
