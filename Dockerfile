#Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

#Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

#Copy source code
COPY main.go ./

#Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o server main.go

#Run stage
FROM alpine:latest

WORKDIR /app

#Copy the binary from builder
COPY --from=builder /app/server .

#Expose port (Cloud Run uses PORT env var)
EXPOSE 8080

#Run the server
CMD ["./server"]