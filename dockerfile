# Stage 1: Build Stage
FROM golang:1.23 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy only the dependency files first to cache layers
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the application binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app .

# Stage 2: Runtime Stage
FROM alpine:latest

# Install necessary dependencies
RUN apk add --no-cache ca-certificates bash

# Set the working directory inside the runtime container
WORKDIR /app

# Copy the compiled binary from the build stage
COPY --from=builder /app/app .

# Copy any necessary config files (if they exist)
COPY --from=builder /app/config ./config

# Expose the application port
EXPOSE 9090

# Define the command to run your application
CMD ["./app"]
