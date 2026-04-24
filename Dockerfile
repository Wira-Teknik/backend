# Build Stage
FROM golang:1.25.5-alpine AS builder

# Install git & swag untuk generate docs jika diperlukan
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod dan sum dulu agar layer caching efisien
COPY go.mod go.sum ./
RUN go mod download

# Copy seluruh source code
COPY . .

# Build aplikasi (CGO_ENABLED=0 agar binary bisa jalan di alpine)
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Final Stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary dari builder stage
COPY --from=builder /app/main .
# Copy folder docs untuk swagger
COPY --from=builder /app/docs ./docs

# Expose port sesuai .env (7001)
EXPOSE 7001

# Jalankan aplikasi
CMD ["./main"]
