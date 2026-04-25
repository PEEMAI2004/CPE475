# Stage 1: Build React UI
FROM node:22-alpine AS frontend-builder
WORKDIR /app/frontend
ARG VITE_GOOGLE_CLIENT_ID
ENV VITE_GOOGLE_CLIENT_ID=$VITE_GOOGLE_CLIENT_ID
COPY frontend/package*.json ./
RUN npm install
COPY frontend/ .
RUN npm run build

# Stage 2: Build Go Manager API
FROM golang:1.25-alpine AS go-builder
WORKDIR /app/manager-api
COPY manager-api/go.mod manager-api/go.sum ./
RUN go mod download
COPY manager-api/ .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o manager ./cmd/manager

# Stage 3: Final Runner 
FROM alpine:latest
WORKDIR /opt/potbuddy/manager-api

# Pull both the unified executable and the static web files
COPY --from=go-builder /app/manager-api/manager ./
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist

CMD ["./manager"]
