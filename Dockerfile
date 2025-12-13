#Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
#Copy go mod files
COPY go.mod go.sum ./
RUN go mod download
#Copy all source code
COPY *.go ./
#Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o server .
#Run stage
FROM alpine:latest
WORKDIR /app
#Install k6 for load testing
RUN apk add --no-cache ca-certificates wget && \
    wget -q https://github.com/grafana/k6/releases/download/v0.48.0/k6-v0.48.0-linux-amd64.tar.gz && \
    tar -xzf k6-v0.48.0-linux-amd64.tar.gz && \
    mv k6-v0.48.0-linux-amd64/k6 /usr/local/bin/ && \
    rm -rf k6-v0.48.0-linux-amd64* && \
    apk del wget
#Copy the binary from builder
COPY --from=builder /app/server .
#Expose port ,Cloud Run uses PORT env var
EXPOSE 8080
#Run the server
CMD ["./server"]
