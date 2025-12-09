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

test:
	go test ./... -v

kafka-init:
	docker exec -it imageprocessor_kafka_1 kafka-topics --create --topic image-processing --bootstrap-server kafka:9092 --partitions 3 --replication-factor 1
	docker exec -it imageprocessor_kafka_1 kafka-topics --create --topic image-processed --bootstrap-server kafka:9092 --partitions 3 --replication-factor 1

migrate-up:
	goose -dir migrations postgres "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable" up

migrate-down:
	goose -dir migrations postgres "postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=disable" down