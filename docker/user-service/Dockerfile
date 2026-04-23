FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/user-service ./cmd/user-service

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/user-service /bin/user-service
EXPOSE 8082
ENTRYPOINT ["/bin/user-service"]
