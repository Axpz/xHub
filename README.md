# xHub - Agent Management System

xHub是一个基于Gin和gRPC的分布式Agent管理系统，支持Agent注册、心跳检测、任务分发和结果收集。

## 功能特性

- **Agent注册**: 支持Agent自动注册到服务器
- **心跳机制**: 实时监控Agent状态
- **任务分发**: 支持向Agent分发任务
- **结果收集**: 收集Agent执行任务的结果
- **REST API**: 提供HTTP接口进行管理
- **gRPC通信**: 高效的二进制通信协议
- **可扩展**: 支持无限扩展Agent客户端

## 架构设计

```
┌─────────────┐    HTTP API    ┌─────────────┐
│   Client    │ ──────────────▶│   Server    │
│  (Browser)  │                │  (xHub)     │
└─────────────┘                └─────────────┘
                                      │
                                      │ gRPC
                                      ▼
                               ┌─────────────┐
                               │   Agent 1   │
                               └─────────────┘
                                      │
                               ┌─────────────┐
                               │   Agent 2   │
                               └─────────────┘
                                      │
                               ┌─────────────┐
                               │   Agent N   │
                               └─────────────┘
```

## 快速开始

### 前置要求

- Go 1.24+
- Protocol Buffers编译器

### 安装依赖

```bash
# 安装protoc (macOS)
make install-protoc-mac

# 安装protoc (Ubuntu/Debian)
make install-protoc-ubuntu

# 安装Go protobuf插件
make install-go-protoc

# 更新Go依赖
make tidy
```

### 构建项目

```bash
# 生成protobuf代码并构建
make build-all

# 或者分别构建
make build      # 构建服务器
make build-agent # 构建Agent客户端
```

### 运行服务

```bash
# 启动服务器
make run

# 或者开发模式运行
make dev

# 启动Agent客户端
make run-agent

# 或者开发模式运行Agent
make dev-agent
```

## API接口

### HTTP API

服务器启动后，HTTP API将在 `http://localhost:8080` 提供服务。

#### Agent管理

- `GET /api/v1/agents` - 获取所有Agent列表
- `GET /api/v1/agents/:id` - 获取指定Agent信息
- `DELETE /api/v1/agents/:id` - 删除指定Agent

#### 任务管理

- `GET /api/v1/tasks` - 获取所有任务列表
- `POST /api/v1/tasks` - 创建新任务
- `GET /api/v1/tasks/:id` - 获取指定任务信息
- `DELETE /api/v1/tasks/:id` - 删除指定任务

#### 系统状态

- `GET /api/v1/status` - 获取系统状态
- `GET /health` - 健康检查

#### 创建任务示例

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "shell",
    "command": "echo hello world",
    "parameters": {"timeout": "30"},
    "timeout_seconds": 30
  }'
```

### gRPC API

gRPC服务在 `localhost:9090` 提供服务。

#### 服务定义

- `Register` - Agent注册
- `Heartbeat` - 心跳检测
- `GetTask` - 获取任务
- `SubmitTaskResult` - 提交任务结果
- `StreamTask` - 流式任务处理

## 项目结构

```
xHub/
├── cmd/
│   ├── main.go          # 服务器入口
│   └── agent/
│       └── main.go      # Agent客户端入口
├── internal/
│   ├── models/
│   │   └── agent.go     # 数据模型
│   ├── server/
│   │   ├── grpc_server.go # gRPC服务器
│   │   └── http_server.go # HTTP服务器
│   └── store/
│       └── store.go     # 数据存储
├── proto/
│   └── agent.proto      # Protocol Buffers定义
├── data/                # 数据存储目录
├── bin/                 # 构建输出目录
├── Makefile            # 构建脚本
├── go.mod              # Go模块文件
└── README.md           # 项目文档
```

## 开发指南

### 添加新的任务类型

1. 在 `internal/models/agent.go` 中定义新的任务类型
2. 在Agent客户端中实现任务执行逻辑
3. 更新protobuf定义（如需要）

### 扩展API接口

1. 在 `internal/server/http_server.go` 中添加新的路由
2. 在 `internal/server/grpc_server.go` 中添加新的gRPC方法
3. 更新protobuf定义（如需要）

### 自定义存储后端

实现 `internal/store/store.go` 中的接口，支持数据库存储。

## 配置

### 环境变量

- `HTTP_PORT` - HTTP服务器端口（默认：8080）
- `GRPC_PORT` - gRPC服务器端口（默认：9090）

### 数据存储

默认使用JSON文件存储，数据保存在 `./data/` 目录下。

## 故障排除

### 常见问题

1. **protoc命令未找到**
   ```bash
   make install-protoc-mac  # macOS
   make install-protoc-ubuntu  # Ubuntu/Debian
   ```

2. **Agent连接失败**
   - 检查服务器是否正在运行
   - 确认gRPC端口（9090）是否可访问
   - 检查网络连接

3. **任务执行失败**
   - 检查Agent是否已注册
   - 查看Agent日志输出
   - 确认任务参数是否正确

## 贡献

欢迎提交Issue和Pull Request！

## 许可证

MIT License 