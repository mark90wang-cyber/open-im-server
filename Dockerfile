# Use Go 1.25 Alpine as the base image for building the application
FROM golang:1.25-alpine AS builder

# Define the base directory for the application as an environment variable
ENV SERVER_DIR=/openim-server

# Set the working directory inside the container based on the environment variable
WORKDIR $SERVER_DIR

# Set the Go proxy to improve dependency resolution speed
# ENV GOPROXY=https://goproxy.io,direct

# Install Mage first (cached independently of source code changes)
RUN go install github.com/magefile/mage@v1.15.0

# Copy all files from the current directory into the container
COPY . .

# Use vendor directory if present, otherwise download dependencies
RUN if [ -d "vendor" ]; then echo "Using vendor directory"; else go mod download; fi
ENV GOFLAGS=-mod=vendor

# Optionally build your application if needed
RUN mage build

# Using Alpine Linux with Go environment for the final image
FROM golang:1.25-alpine

# Install necessary packages, such as bash
RUN apk add --no-cache bash

# Set the environment and work directory
ENV SERVER_DIR=/openim-server
WORKDIR $SERVER_DIR


# Copy the compiled binaries and mage from the builder image to the final image
COPY --from=builder $SERVER_DIR/_output $SERVER_DIR/_output
COPY --from=builder $SERVER_DIR/config $SERVER_DIR/config
COPY --from=builder /go/bin/mage /usr/local/bin/mage
COPY --from=builder $SERVER_DIR/magefile_windows.go $SERVER_DIR/
COPY --from=builder $SERVER_DIR/magefile_unix.go $SERVER_DIR/
COPY --from=builder $SERVER_DIR/magefile.go $SERVER_DIR/
# magefile.go imports the in-repo version package, which embeds version/version
COPY --from=builder $SERVER_DIR/version $SERVER_DIR/version
COPY --from=builder $SERVER_DIR/start-config.yml $SERVER_DIR/
COPY --from=builder $SERVER_DIR/go.mod $SERVER_DIR/
COPY --from=builder $SERVER_DIR/go.sum $SERVER_DIR/

# Remove local replace directive (only needed at build time) before go get
RUN sed -i '/^replace /d' go.mod && go get github.com/openimsdk/gomake@v0.0.17

# Set the command to run when the container starts
ENTRYPOINT ["sh", "-c", "mage start && tail -f /dev/null"]
