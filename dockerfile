FROM golang:1.24-alpine AS builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o image-processor ./cmd/image-processor
RUN go build -o worker ./cmd/worker

FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /app/image-processor /app/worker /app/
COPY static /app/static
COPY templates /app/templates

EXPOSE 8004

CMD ["./image-processor"]