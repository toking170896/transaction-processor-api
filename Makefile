.PHONY: run test test-e2e swag mockss mocks-clean docker-up docker-down

run: ## Run the application locally
	go run cmd/server/main.go

test: ## Run unit and handler tests (no E2E)
	SKIP_E2E=1 go test -v ./...

test-e2e: ## Run E2E tests (requires DB)
	go test -v ./internal/test/...

swag: ## Regenerates swagger docs
	swag init -g cmd/server/main.go --parseDependency --parseInternal

mockss: ## Regenerate mocks
	mockery --all --dir="./internal" --output "./mocks" --keeptree

mocks-clean: ## Clean and regenerate mocks
	rm -rf ./mocks
	$(MAKE) mockss

docker-up: ## Start the application
	docker-compose up --build

docker-down: ## Stop the application
	docker-compose down