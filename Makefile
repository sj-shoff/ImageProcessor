.PHONY: run run-worker build migrate-up migrate-down docker-up docker-down

include .env
export

run:
	go run cmd/image-processor/main.go

run-worker:
	go run cmd/worker/main.go

build:
	go build -o bin/image-processor cmd/image-processor/main.go
	go build -o bin/worker cmd/worker/main.go

docker-up:
	docker-compose up -d --build
	
docker-down:
	docker-compose down

migrate-up:
	goose -dir migrations postgres "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable" up

migrate-down:
	goose -dir migrations postgres "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable" down