FROM golang:1.25-alpine AS builder

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build the binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /transaction-processor ./cmd/server/main.go

FROM alpine:3.19

# Run only the compiled binary (no build tools or source code)
COPY --from=builder /transaction-processor /transaction-processor
EXPOSE 8080
CMD ["/transaction-processor"]
