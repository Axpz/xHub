.PHONY: all build clean run dev tidy proto server agent test help

# 默认目标
all: proto build

# 生成protobuf代码
proto:
	@echo "Generating protobuf code..."
	@mkdir -p proto/agent
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/agent.proto

# 构建服务器
build: proto
	@echo "Building server..."
	go build -o bin/xhub-server cmd/main.go

# 构建agent客户端
build-agent: proto
	@echo "Building agent client..."
	go build -o bin/xhub-agent cmd/agent/main.go

# 构建所有
build-all: build build-agent

# 运行服务器
run: build
	@echo "Running server..."
	./bin/xhub-server

# 运行agent客户端
run-agent: build-agent
	@echo "Running agent client..."
	./bin/xhub-agent

# 开发模式运行服务器
dev: proto
	@echo "Running server in development mode..."
	go run cmd/main.go

# 开发模式运行agent
dev-agent: proto
	@echo "Running agent in development mode..."
	go run cmd/agent/main.go

# 清理构建文件
clean:
	@echo "Cleaning build files..."
	rm -rf bin/
	rm -f xHub
	rm -f xhub-server
	rm -f xhub-agent

# 更新依赖
tidy:
	@echo "Updating dependencies..."
	go mod tidy

# 运行测试
test:
	@echo "Running tests..."
	go test ./...

# 安装protoc工具（macOS）
install-protoc-mac:
	@echo "Installing protoc on macOS..."
	brew install protobuf

# 安装protoc工具（Ubuntu/Debian）
install-protoc-ubuntu:
	@echo "Installing protoc on Ubuntu/Debian..."
	sudo apt update
	sudo apt install -y protobuf-compiler

# 安装Go protobuf插件
install-go-protoc:
	@echo "Installing Go protobuf plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 创建必要的目录
setup:
	@echo "Setting up project structure..."
	mkdir -p bin
	mkdir -p data
	mkdir -p proto/agent

# 显示帮助信息
help:
	@echo "Available commands:"
	@echo "  proto           - Generate protobuf code"
	@echo "  build           - Build server"
	@echo "  build-agent     - Build agent client"
	@echo "  build-all       - Build server and agent"
	@echo "  run             - Run server"
	@echo "  run-agent       - Run agent client"
	@echo "  dev             - Run server in development mode"
	@echo "  dev-agent       - Run agent in development mode"
	@echo "  clean           - Clean build files"
	@echo "  tidy            - Update dependencies"
	@echo "  test            - Run tests"
	@echo "  setup           - Create necessary directories"
	@echo "  install-protoc-mac    - Install protoc on macOS"
	@echo "  install-protoc-ubuntu - Install protoc on Ubuntu/Debian"
	@echo "  install-go-protoc     - Install Go protobuf plugins"