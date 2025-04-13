# Use the official Golang 1.23 image to build the app
FROM golang:1.23-alpine as builder

# Set the current working directory inside the container
WORKDIR /app

# Copy the Go modules and source code into the container
COPY go.mod go.sum ./
RUN go mod tidy

COPY . .

# Build the Go app
RUN go build -o myapp ./cmd/myapp

# Create a minimal final image with only the compiled app
FROM alpine:latest

WORKDIR /root/

# Copy the compiled binary from the builder container
COPY --from=builder /app/myapp .

# Expose the port the app will run on
EXPOSE 8080

# Run the Go application
CMD ["./myapp"]