PROJECT=cpustat

BUILD_PATH := $(shell pwd)/.gobuild

BIN := $(PROJECT)
SOURCE_PATH := $(BUILD_PATH)/src/github.com/hectorj2f
SOURCE=$(shell find . -name '*.go')

VERSION := $(shell cat VERSION)
COMMIT := $(shell git rev-parse --short HEAD)

GOPATH := $(BUILD_PATH)
GOVERSION=1.5
ifndef GOOS
	GOOS := linux
endif
ifndef GOARCH
	GOARCH := amd64
endif

.PHONY: all .gobuild BIN

all: .gobuild $(BIN)

.gobuild:
	@mkdir -p $(SOURCE_PATH)
	@rm -f $(SOURCE_PATH)/$(PROJECT) && cd "$(SOURCE_PATH)" && ln -s ../../../.. $(PROJECT)

	@GOPATH=$(GOPATH) go get github.com/codahale/hdrhistogram
	@GOPATH=$(GOPATH) go get github.com/remyoudompheng/go-netlink
	@GOPATH=$(GOPATH) go get github.com/remyoudompheng/go-netlink/genl
	@GOPATH=$(GOPATH) go get github.com/uber-common/termui

$(BIN): $(SOURCE) VERSION
	@echo Building for $(GOOS)/$(GOARCH)
	docker run \
		--rm \
		-v $(shell pwd):/usr/code \
		-e GOPATH=/usr/code/.gobuild \
		-e GOOS=$(GOOS) \
		-e GOARCH=$(GOARCH) \
		-w /usr/code \
		golang:$(GOVERSION) \
		go build -a -ldflags \
		"-X github.com/hectorj2f/cpustat/cli.projectVersion=$(VERSION) -X github.com/hectorj2f/cpustat/cli.projectBuild=$(COMMIT)" \
		-o $(BIN)

clean:
	rm -rf $(BUILD_PATH) $(BIN)
