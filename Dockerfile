# Stage 1: Build the Go binary
FROM golang:1.23-alpine AS builder

# Create the work dir
RUN mkdir -p /app

# Change the working directory
WORKDIR /app

# Copy the rest of the relevant files
COPY go.mod go.sum README.md Makefile container.sh ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the command directory
COPY cmd/ ./cmd/

# Copy internal packages
COPY internal/ ./internal/

# Build the binary
RUN env CGO_ENABLED=0 go build -o ./build/bin/server -ldflags '-s' ./cmd/server/main.go

# Stage 2: Create the final image from scratch
FROM scratch

# Copy CA certificates from the builder image
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the compiled Go binary from the builder stage
COPY --from=builder /app/build/bin/server /go/bin/server

ENTRYPOINT ["/go/bin/server"]
