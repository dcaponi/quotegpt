# syntax=docker/dockerfile:1.2

# Base stage for building the Go application
FROM golang:1.23-alpine AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Install template engine
RUN go install github.com/a-h/templ/cmd/templ@latest

# Generate the templ stuff
RUN templ generate

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 go build -o main cmd/api/main.go


# Development stage with air for hot reloading
FROM golang:1.23-alpine AS dev

# Set the Current Working Directory inside the container
WORKDIR /app

# Install dependencies
RUN apk add --no-cache git curl make g++

# Install air for live reloading
RUN go install github.com/air-verse/air@latest

# Install templ so air can use it to generate new code after changes to .templ files
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code into the container
COPY . .

# Command to run air
CMD ["air"]


# Production stage
FROM alpine:3.15 AS prod

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy the built Go application from the builder stage
COPY --from=builder /app/main /app/main

# Poke a hole for port
EXPOSE 8080

# Command to run the Go application
CMD ["/app/main"]