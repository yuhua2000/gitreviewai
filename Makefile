.PHONY: build build-frontend build-go dev-frontend dev-go clean

# Build frontend
build-frontend:
	cd internal/api/frontend && npm install && npm run build

# Build Go binary (depends on frontend build)
build-go:
	CGO_ENABLED=0 go build -trimpath -o gitreviewai cmd/server/main.go

# Full build
build: build-frontend build-go

# Development: run frontend dev server
dev-frontend:
	cd internal/api/frontend && npm run dev

# Development: run Go backend
dev-go:
	go run cmd/server/main.go

# Clean
clean:
	rm -rf gitreviewai internal/api/frontend/dist internal/api/frontend/node_modules data/
