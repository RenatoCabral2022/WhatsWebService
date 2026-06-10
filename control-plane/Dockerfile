FROM golang:1.25-alpine AS builder
WORKDIR /app

# Cache module download by copying only manifests first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/control-plane ./cmd/server

FROM alpine:3.19
RUN apk --no-cache add ca-certificates
COPY --from=builder /bin/control-plane /bin/control-plane
# Cloud Run / Cloud Functions 2nd gen sets PORT; main.go reads it via config.
EXPOSE 8080
ENTRYPOINT ["/bin/control-plane"]
