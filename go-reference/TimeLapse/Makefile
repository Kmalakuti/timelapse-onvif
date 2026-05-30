.PHONY: help build up down test shell clean

help: ## Show this help message
	@echo "TimeLapse Development Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build Docker image
	docker-compose build

up: ## Start development environment
	docker-compose up

down: ## Stop development environment
	docker-compose down

test: ## Run all tests
	docker-compose run --rm timelapse-dev go test -v ./...

test-coverage: ## Run tests with coverage
	docker-compose run --rm timelapse-dev go test -cover ./...

shell: ## Open shell in container
	docker-compose run --rm timelapse-dev /bin/bash

clean: ## Clean up containers and volumes
	docker-compose down -v
	rm -rf data/*

init: ## Initialize project (download dependencies)
	docker-compose run --rm timelapse-dev go mod download
