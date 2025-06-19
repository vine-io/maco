NAME=maco
IMAGE_NAME=vine-io/$(NAME)
GIT_COMMIT=$(shell git rev-parse --short HEAD)
GIT_TAG=$(shell git describe --abbrev=0 --tags --always --match "v*")
GIT_VERSION=github.com/vine-io/maco/pkg/version
CGO_ENABLED=0
BUILD_DATE=$(shell date +%s)
LDFLAGS=-X $(GIT_VERSION).GitCommit=$(GIT_COMMIT) -X $(GIT_VERSION).GitTag=$(GIT_TAG) -X $(GIT_VERSION).BuildDate=$(BUILD_DATE)
IMAGE_TAG=$(GIT_TAG)-$(GIT_COMMIT)
ROOT=github.com/vine-io/maco

ifeq ($(GOHOSTOS), windows)
        #the `find.exe` is different from `find` in bash/shell.
        #to see https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/find.
        #changed to use git-bash.exe to run find cli or other cli friendly, caused of every developer has a Git.
        #Git_Bash= $(subst cmd\,bin\bash.exe,$(dir $(shell where git)))
        Git_Bash=$(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
        TYPES_PROTO_FILES=$(shell $(Git_Bash) -c "find api/types -name *.proto")
        RPC_PROTO_FILES=$(shell $(Git_Bash) -c "find api/rpc -name *.proto")
else
        TYPES_PROTO_FILES=$(shell find api/types -name *.proto)
        RPC_PROTO_FILES=$(shell find api/rpc -name *.proto)
endif



all: build

vendor:
	go mod vendor

test-coverage:
	go test ./... -bench=. -coverage

lint:
	golint -set_exit_status ./..

install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	go install github.com/envoyproxy/protoc-gen-validate@latest


apis:
	protoc --proto_path=. --go_out=paths=source_relative:. api/errors/errors.proto
	protoc --proto_path=. \
			--proto_path=./third-party/google/protobuf \
			--go_out=paths=source_relative:. \
			$(TYPES_PROTO_FILES)
	protoc --proto_path=. \
    		--proto_path=./third-party/google/protobuf \
    		--go_out=paths=source_relative:. \
    		--go-grpc_out=paths=source_relative:. \
    		--validate_out=paths=source_relative,lang=go:. \
    		--grpc-gateway_out=paths=source_relative:. \
    		--openapi_out=fq_schema_naming=true,title="maco",description="maco OpenAPI3.0 Document",version=$(GIT_TAG),default_response=false:./docs \
    		$(RPC_PROTO_FILES)

docker:


vet:
	go vet ./...

test: vet
	go test -v ./...

clean:
	rm -fr ./_output

