FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o image-processor ./cmd/image-processor/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -o worker ./cmd/worker/main.go

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata curl postgresql-client

COPY --from=builder /app/image-processor /app/worker /app/
COPY static /app/static

EXPOSE 8034

CMD ["./image-processor"]