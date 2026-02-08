FROM golang:1.22-alpine AS builder
RUN apk add --no-cache build-base opus-dev pkgconf git
WORKDIR /app

# Copy proto stubs module (replace directive target)
COPY gen/go/ gen/go/

# Copy gateway source
COPY webrtc-gateway/ webrtc-gateway/

WORKDIR /app/webrtc-gateway
RUN go mod tidy && CGO_ENABLED=1 GOOS=linux go build -tags nolibopusfile -o /bin/gateway ./cmd/gateway

FROM alpine:3.19
RUN apk --no-cache add ca-certificates opus ffmpeg
COPY --from=builder /bin/gateway /bin/gateway
EXPOSE 9090 9091 9092
ENTRYPOINT ["/bin/gateway"]
