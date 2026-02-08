module github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway

go 1.22

require (
	github.com/RenatoCabral2022/WhatsWebService/gen/go v0.0.0
	github.com/hraban/opus v0.0.0-20251117090126-c76ea7e21bf3
	github.com/pion/interceptor v0.1.37
	github.com/pion/webrtc/v4 v4.0.10
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.70.0
	google.golang.org/protobuf v1.36.4
)

require (
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
)

replace github.com/RenatoCabral2022/WhatsWebService/gen/go => ../gen/go
