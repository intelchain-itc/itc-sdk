SHELL := /bin/bash
TOP:=$(realpath ..)
export LD_LIBRARY_PATH:=$(TOP)/bls/lib:$(TOP)/mcl/lib:/usr/local/opt/openssl/lib:/opt/homebrew/opt/gmp/lib/:/opt/homebrew/opt/openssl/lib
export LIBRARY_PATH:=$(LD_LIBRARY_PATH)
export CXXFLAGS += -Wno-deprecated-declarations  
version := $(shell git rev-list --count HEAD)
commit := $(shell git describe --always --long --dirty)
built_at := $(shell date +%FT%T%z)
built_by := ${USER}@intelchain.org

flags := -gcflags="all=-N -l"
ldflags := -X main.version=v${version} -X main.commit=${commit}
ldflags += -X main.builtAt=${built_at} -X main.builtBy=${built_by}
cli := ./dist/itc
upload-path-darwin := 's3://pub.intelchain.org/release/darwin-x86_64/mainnet/itc'
upload-path-linux := 's3://pub.intelchain.org/release/linux-x86_64/mainnet/itc'
upload-path-linux-version := 's3://pub.intelchain.org/release/linux-x86_64/mainnet/itc_version'
uname := $(shell uname)

env := GO111MODULE=on

DIR := ${CURDIR}
export CGO_LDFLAGS=-L$(DIR)/dist/lib -Wl,-rpath -Wl,\$ORIGIN/lib

all:
	source $(shell go env GOPATH)/src/github.com/intelchain-itc/intelchain/scripts/setup_bls_build_flags.sh && $(env) go build -o $(cli) -ldflags="$(ldflags)" cmd/main.go
	cp $(cli) itc

static:
	make -C $(shell go env GOPATH)/src/github.com/intelchain-itc/mcl
	make -C $(shell go env GOPATH)/src/github.com/intelchain-itc/bls minimised_static BLS_SWAP_G=1
	source $(shell go env GOPATH)/src/github.com/intelchain-itc/intelchain/scripts/setup_bls_build_flags.sh && $(env) go build -o $(cli) -ldflags="$(ldflags) -w -extldflags \"-static\"" cmd/main.go
	cp $(cli) itc

debug:
	source $(shell go env GOPATH)/src/github.com/intelchain-itc/intelchain/scripts/setup_bls_build_flags.sh && $(env) go build $(flags) -o $(cli) -ldflags="$(ldflags)" cmd/main.go
	cp $(cli) itc

install:all
	cp $(cli) ~/.local/bin

run-tests: test-rpc test-key;

test-key:
	go test ./pkg/keys -cover -v

test-rpc:
	go test ./pkg/rpc -cover -v

# Notice assumes you have correct uploading credentials
upload-darwin:all
ifeq (${uname}, Darwin)
	aws --profile upload s3 cp ./itc ${upload-path-darwin}
else
	@echo "Wrong operating system for target upload"
endif

# Only the linux build will upload the CLI version
upload-linux:static
ifeq (${uname}, Linux)
	aws --profile upload s3 cp ./itc ${upload-path-linux}
	./itc version &> ./itc_version
	aws --profile upload s3 cp ./itc_version ${upload-path-linux-version}
else
	@echo "Wrong operating system for target upload"
endif

.PHONY:clean run-tests upload-darwin upload-linux

clean:
	@rm -f $(cli)
	@rm -rf ./dist
