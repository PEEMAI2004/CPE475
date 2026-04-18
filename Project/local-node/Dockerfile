# Build Stage
FROM golang:1.24-alpine AS builder
WORKDIR /app

# Download dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy application source
COPY . .

# Compile statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o node ./cmd/node

# Run Stage
FROM alpine:latest
WORKDIR /opt/potbuddy/local-node

# Create directory structure
RUN mkdir -p /opt/potbuddy/local-node/bin

# Copy outputs from the builder stage
COPY --from=builder /app/node ./bin/
COPY --from=builder /app/config.yaml ./

# Ensure correct execution environment
CMD ["./bin/node"]
