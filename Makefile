.PHONY: help build build-no-cache push test test-race test-cover test-smoke

IMAGE_NAME = ghcr.io/mimic890/kuraban
IMAGE_TAG = latest

## help: Show this help message
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## build: Build the Docker image locally
build:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

## build-no-cache: Build the Docker image without cache
build-no-cache:
	docker build --no-cache -t $(IMAGE_NAME):$(IMAGE_TAG) .

## push: Build and push the Docker image to the registry
push: build
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

## test: Run backend unit tests
test:
	cd backend && go test ./...

## test-race: Run backend unit tests with the race detector
test-race:
	cd backend && go test -race ./...

## test-cover: Run backend unit tests with coverage and enforce the coverage gate
test-cover:
	cd backend && go test -coverprofile=../coverage.out -coverpkg=./... ./...
	bash scripts/coverage-gate.sh coverage.out

## test-smoke: Run the docker compose smoke test (requires Docker + compose v2)
test-smoke:
	cd backend && go test -tags=smoke -timeout 10m ./tests/smoke/...

.DEFAULT_GOAL := help
