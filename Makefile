VERSION := $(shell git describe --tags)
BUILD := $(shell git rev-parse --short HEAD)
PROJECT := $(shell basename "$(PWD)")

GOBASE := $(shell pwd)
GOBIN := $(GOBASE)/bin
GOOS := "linux"
GOARCH := "amd64"

install:
	@go mod tidy && go mod vendor
start:
	@air -c .air.toml
build:
	@echo ">  Building server"
	@CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags -X=main.version=$(VERSION) -o bin/turn_server cmd/server/main.go
build-client:
	@echo ">  Building client"
	@CGO_ENABLED=0 GOOS=darwin GOARCH=$(GOARCH) go build -ldflags -X=main.version=$(VERSION) -o bin/turn_client cmd/client/main.go
test:
	@go test -v ./...
upload:
	@ssh root@jp.252798.xyz "systemctl stop wrtc_turn"
	@scp -C wrtc_turn config.yml root@jp.252798.xyz:/root/turn/
	@ssh root@jp.252798.xyz "systemctl start wrtc_turn"
upload-test:
	@ssh root@27.128.112.246 "systemctl stop wrtc_turn"
	@scp -C bin/turn_server root@27.128.112.246:/root/turn/
	@ssh root@27.128.112.246 "systemctl start wrtc_turn"

.PHONY: install build upload test
