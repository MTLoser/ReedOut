.PHONY: dev dev-backend dev-frontend build clean deps

# Development: run backend and frontend concurrently
dev:
	@echo "Starting ReedOut development servers..."
	@echo "  Backend:  http://localhost:8080"
	@echo "  Frontend: http://localhost:5173"
	@make -j2 dev-backend dev-frontend

dev-backend:
	go run ./cmd/reedout

dev-frontend:
	cd web && npm run dev

# Build production binary with embedded frontend
build:
	cd web && npm ci && npm run build
	CGO_ENABLED=1 go build -o bin/reedout ./cmd/reedout

# Install all dependencies
deps:
	go mod tidy
	cd web && npm install

clean:
	rm -rf bin/
	rm -rf web/dist/
	rm -rf data/
