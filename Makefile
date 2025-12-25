PROJECT_NAME=kube-nova
MAIN_FILE=main.go
PKG="github.com/yanshicheng/$(PROJECT_NAME)"
APPLICATION_DIR := $(shell pwd)/application
TEMPLATE_HOME="./template"
REGISTRY_UEL="harbor.ikubeops.local/public"
IMAGE_TIMESTAMP := ${shell date +%Y%m%d%H%M%S}

MOD_DIR := $(shell go env GOMODCACHE)
PKG_LIST := $(shell go list ${PKG}/... | grep -v /vendor/)
GO_FILES := $(shell find . -name '*.go' | grep -v /vendor/ | grep -v _test.go)

GIT_TAG := ${shell git describe --tags --abbrev=0 2>/dev/null || echo 'v0.0.1'}
GIT_TAGS := ${shell git describe --tags $(git rev-list --tags --max-count=1) 2>/dev/null || echo v0.0.2}
GIT_URL := $(shell git config --get remote.origin.url | awk '{sub(/\.git$$/, ""); print}')
BUILD_BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
BUILD_COMMIT := ${shell git rev-parse --short=10 HEAD}
BUILD_TIME := ${shell date '+%Y-%m-%d %H:%M:%S'}
BUILD_GO_VERSION := $(shell go version | grep -o  'go[0-9].[0-9].*')
VERSION_PATH := "${PKG}/version"

.PHONY: all run tidy lint fmt vet test test-coverage build clean

all: build

tidy: ## 自动更新依赖
	@echo "正在更新依赖..."
	@go mod tidy

lint: ## 检测代码格式
	@golangci-lint run

fmt: ## 正在格式化代码
	@echo "正在格式化代码..."
	@go fmt ${PKG_LIST}

vet: ## 检测代码错误
	@echo "正在检测代码错误..."
	@go vet ${PKG_LIST}

test: ## Run tests
	@echo "正在测试代码..."
	@go test -short ${PKG_LIST}

test-coverage: ## 测试覆盖率
	@echo "正在测试代码覆盖率..."
	@go test -short -coverprofile cover.out -covermode=atomic ${PKG_LIST}
	@cat cover.out >> coverage.txt

build: tidy ## Build the binary file
	@echo "正在构建二进制文件..."
	@CGO_ENABLED=0  go build -ldflags "-s -w" \
	-ldflags "-X '${VERSION_PATH}.IkubeopsBranch=${BUILD_BRANCH}' \
	-X '${VERSION_PATH}.IkubeopsCommit=${BUILD_COMMIT}' \
	-X '${VERSION_PATH}.IkubeopsBuildTime=${BUILD_TIME}' \
	-X '${VERSION_PATH}.IkubeopsGoVersion=${BUILD_GO_VERSION}' \
	-X '${VERSION_PATH}.IkubeopsTag=${GIT_TAGS}' \
	-X '${VERSION_PATH}.IkubeopsProjectName=${PROJECT_NAME}' \
	-X '${VERSION_PATH}.IkubeopsUrl=${GIT_URL}' " \
	-o dist/$(PROJECT_NAME) $(MAIN_FILE)
	@cp -r config dist/

linux: tidy ## 构建linux版本
	@echo "正在构建linux amd64 版本..."
	@CGO_ENABLED=0  GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" \
	-ldflags "-X '${VERSION_PATH}.IkubeopsBranch=${BUILD_BRANCH}' \
	-X '${VERSION_PATH}.IkubeopsCommit=${BUILD_COMMIT}' \
	-X '${VERSION_PATH}.IkubeopsBuildTime=${BUILD_TIME}' \
	-X '${VERSION_PATH}.IkubeopsGoVersion=${BUILD_GO_VERSION}' \
	-X '${VERSION_PATH}.IkubeopsTag=${GIT_TAGS}' \
	-X '${VERSION_PATH}.IkubeopsProjectName=${PROJECT_NAME}' \
	-X '${VERSION_PATH}.IkubeopsUrl=${GIT_URL}' " \
	-o dist/$(PROJECT_NAME) $(MAIN_FILE)
	@cp -r config dist/

run: fmt vet ## run server
	@echo "正在运行启动 server..."
	@go run $(MAIN_FILE) start -f config/config.yaml

clean: ## Remove previous build
	@rm -f dist/*

docker: ## 构建docker镜像
	@echo "正在构建docker镜像..."
	@docker build -t ${REGISTRY_UEL}/$(PROJECT_NAME):$(GIT_TAG)-$(IMAGE_TIMESTAMP) .
	@echo "镜像构建完成，正在上传到镜像仓库..."
	@docker push ${REGISTRY_UEL}/$(PROJECT_NAME):$(GIT_TAG)-$(IMAGE_TIMESTAMP)
	@echo "镜像推送完成，镜像: ${REGISTRY_UEL}/$(PROJECT_NAME):$(GIT_TAG)-$(IMAGE_TIMESTAMP)"



gen-portal-api: ## fmt vet 生成 portal-api 服务代码
	@echo "正在生成 portal-api 服务代码..."
	@goctl api go --api=$(APPLICATION_DIR)/portal-api/portal.api --dir=$(APPLICATION_DIR)/portal-api/ --style=goZero --home=$(TEMPLATE_HOME)


run-portal-api:  ## fmt vet 运行 portal-api 服务
	@echo "正在运行 portal-api 服务..."
	@go run $(APPLICATION_DIR)/portal-api/portal.go  -f $(APPLICATION_DIR)/portal-api/etc/portal-api.yaml


##-I=$(go list -f '{{.Dir}}' -m github.com/zeromicro/go-zero)
gen-portal-rpc:
	@echo "正在生成 portal-rpc 服务代码..."
	@goctl rpc protoc --style=goZero --home=$(TEMPLATE_HOME) \
       -I=$(APPLICATION_DIR)/portal-rpc/ \
       --go_out=$(APPLICATION_DIR)/portal-rpc/pb/ \
       --go_opt=module="github.com/yanshicheng/kube-nova/application/portal-rpc/pb" \
       --go-grpc_out=$(APPLICATION_DIR)/portal-rpc/pb/ \
       --go-grpc_opt=module="github.com/yanshicheng/kube-nova/application/portal-rpc/pb"  \
       --zrpc_out=$(APPLICATION_DIR)/portal-rpc/ -m \
       $(APPLICATION_DIR)/portal-rpc/portal.proto



run-portal-rpc: ## fmt vet 运行 portal-rpc 服务
	@echo "正在运行 portal-rpc 服务..."
	@go run $(APPLICATION_DIR)/portal-rpc/portal.go  -f $(APPLICATION_DIR)/portal-rpc/etc/portal.yaml

gen-manager-api:
	@echo "正在生成 manager-api 服务代码..."
	@goctl api go --api=$(APPLICATION_DIR)/manager-api/manager.api --dir=$(APPLICATION_DIR)/manager-api/ --style=goZero --home=$(TEMPLATE_HOME)


run-manager-api: ## fmt vet 运行 manager-api 服务
	@echo "正在运行 manager-api 服务..."
	@go run $(APPLICATION_DIR)/manager-api/manager.go  -f $(APPLICATION_DIR)/manager-api/etc/manager-api.yaml


gen-manager-rpc:
	@echo "正在生成 manager-rpc 服务代码..."
	@goctl rpc protoc --style=goZero --home=$(TEMPLATE_HOME) \
       -I=$(APPLICATION_DIR)/manager-rpc/ \
       --go_out=$(APPLICATION_DIR)/manager-rpc/pb/ \
       --go_opt=module="github.com/yanshicheng/kube-nova/application/manager-rpc/pb" \
       --go-grpc_out=$(APPLICATION_DIR)/manager-rpc/pb/ \
       --go-grpc_opt=module="github.com/yanshicheng/kube-nova/application/manager-rpc/pb"  \
       --zrpc_out=$(APPLICATION_DIR)/manager-rpc/ -m \
       $(APPLICATION_DIR)/manager-rpc/manager.proto

run-manager-rpc: ## fmt vet 运行 manager-rpc 服务
	@echo "正在运行 manager-rpc 服务..."
	@go run $(APPLICATION_DIR)/manager-rpc/manager.go  -f $(APPLICATION_DIR)/manager-rpc/etc/manager.yaml

gen-workload-api:
	@echo "正在生成 workload-api 服务代码..."
	@goctl api go --api=$(APPLICATION_DIR)/workload-api/workload.api --dir=$(APPLICATION_DIR)/workload-api/ --style=goZero --home=$(TEMPLATE_HOME)


run-workload-api: ## fmt vet 运行workload-api 服务
	@echo "正在运行 workload-api 服务..."
	@go run $(APPLICATION_DIR)/workload-api/workload.go  -f $(APPLICATION_DIR)/workload-api/etc/workload-api.yaml


gen-console-api:
	@echo "正在生成 console-api 服务代码..."
	@goctl api go --api=$(APPLICATION_DIR)/console-api/console.api --dir=$(APPLICATION_DIR)/console-api/ --style=goZero --home=$(TEMPLATE_HOME)


run-console-api: ## fmt vet 运行workload-api 服务
	@echo "正在运行 console-api 服务..."
	@go run $(APPLICATION_DIR)/console-api/console.go  -f $(APPLICATION_DIR)/console-api/etc/console-api.yaml

gen-console-rpc:
	@echo "正在生成 console-rpc 服务代码..."
	@goctl rpc protoc --style=goZero --home=$(TEMPLATE_HOME) \
		-I=$(APPLICATION_DIR)/console-rpc \
		--go_out=$(APPLICATION_DIR)/console-rpc/pb/ \
		--go_opt=module="github.com/yanshicheng/kube-nova/application/console-rpc/pb" \
		--go-grpc_out=$(APPLICATION_DIR)/console-rpc/pb/ \
		--go-grpc_opt=module="github.com/yanshicheng/kube-nova/application/console-rpc/pb" \
		--zrpc_out=$(APPLICATION_DIR)/console-rpc -m \
		$(APPLICATION_DIR)/console-rpc/console.proto

run-console-rpc: ## fmt vet 运行 portal-rpc 服务
	@echo "正在运行 console-rpc 服务..."
	@go run $(APPLICATION_DIR)/console-rpc/console.go  -f $(APPLICATION_DIR)/console-rpc/etc/console.yaml






help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'


#
#	@goctl portal-rpc protoc \
#		$(APPLICATION_DIR)/controller/portal-rpc/proto/*.proto \
#		--go_out=$(APPLICATION_DIR)/controller/portal-rpc \
#		--go-grpc_out=$(APPLICATION_DIR)/controller/portal-rpc \
#		--zrpc_out=$(APPLICATION_DIR)/controller/portal-rpc \
#		--proto_path=$(APPLICATION_DIR)/controller/portal-rpc/proto \
#		--style=goZero
# application/controller/portal-rpc/proto/all.proto
#
#syntax = "proto3";
#
#option go_package = "github.com/yanshicheng/kube-nova/application/controller/portal-rpc/pb";
#
#package pb;
#
#// 导入所有其他 proto 文件
#import "pod.proto";
#import "cluster.proto";
#import "node.proto";
#import "namespace.proto";
#import "common.proto";#
