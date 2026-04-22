FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/transfer-service ./cmd/transfer-service

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/transfer-service /bin/transfer-service
EXPOSE 8081
ENTRYPOINT ["/bin/transfer-service"]
