.PHONY: build build-frontend build-go dev-frontend dev-go clean

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-s -w -X github.com/yuhua2000/gitreviewai/cmd.version=$(VERSION) -X github.com/yuhua2000/gitreviewai/cmd.commit=$(COMMIT)"

# Build frontend
build-frontend:
	cd frontend && npm install && npm run build

# Build Go binary (depends on frontend build)
build-go:
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS) -o gitreviewai .

# Full build
build: build-frontend build-go

# Development: run frontend dev server
dev-frontend:
	cd frontend && npm run dev

# Development: run Go backend
dev-go:
	go run . server

# Clean
clean:
	rm -rf gitreviewai frontend/dist frontend/node_modules data/
