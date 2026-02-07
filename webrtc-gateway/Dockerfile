FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/gateway ./cmd/gateway

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/gateway /bin/gateway
EXPOSE 9090
ENTRYPOINT ["/bin/gateway"]
