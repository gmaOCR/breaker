BIN := bin/breaker
PKG := ./cmd/breaker
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build test e2e vet fmt clean xbuild

build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BIN) $(PKG)

test:
	go test ./...

e2e:
	go test ./test/ -run TestBreakerKills -v

vet:
	go vet ./...

fmt:
	gofmt -l -w .

clean:
	rm -rf bin dist

# Cross-compile static binaries for release.
xbuild:
	@mkdir -p dist
	@for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do \
		os=$${target%/*}; arch=$${target#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		echo "→ dist/breaker-$$os-$$arch$$ext"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -ldflags "-s -w -X main.version=$(VERSION)" \
			-o dist/breaker-$$os-$$arch$$ext $(PKG); \
	done
