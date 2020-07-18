all: backend proxy

backend: cmd/backend.go
	go build cmd/backend.go

proxy: cmd/proxy.go
	go build cmd/proxy.go
