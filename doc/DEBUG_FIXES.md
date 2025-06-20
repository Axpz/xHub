# xHub 调试修复说明

## 问题分析

### 1. 数据存储隔离问题
**问题**: HTTP服务器和gRPC服务器使用了不同的store实例，导致数据完全隔离。

**原因**: 
- `cmd/main.go` 中创建了一个store实例给HTTP服务器
- `server.NewGRPCServer()` 内部又创建了另一个独立的store实例
- 两个store之间无法共享数据

**影响**: 
- HTTP服务器创建的任务保存在一个store中
- gRPC服务器在另一个store中查找任务
- Agent无法获取到通过HTTP接口创建的任务

### 2. 任务分配逻辑问题
**问题**: 任务分配逻辑不够完善，没有考虑agent的在线状态和空闲状态。

**原因**: 
- `hasPendingTask` 和 `findPendingTask` 方法没有检查agent状态
- 可能导致离线或忙碌的agent被分配任务

### 3. 调试信息不足
**问题**: 缺乏详细的日志输出，难以调试问题。

## 修复方案

### 1. 统一数据存储
- 修改 `cmd/main.go`，让HTTP和gRPC服务器共享同一个store实例
- 添加 `NewGRPCServerWithStore()` 构造函数，接受外部传入的store

### 2. 改进任务分配逻辑
- 在 `hasPendingTask` 和 `findPendingTask` 中检查agent状态
- 只给在线且空闲的agent分配任务
- 优先分配专门分配给特定agent的任务

### 3. 增加调试功能
- 添加详细的日志输出
- 新增 `/api/v1/debug` 接口，提供完整的系统状态信息
- 在关键操作点添加日志

## 使用方法

### 1. 启动服务器
```bash
go run cmd/main.go
```

### 2. 查看调试信息
```bash
curl http://localhost:20080/api/v1/debug
```

### 3. 创建任务
```bash
curl -X POST http://localhost:20080/api/v1/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "type": "test",
    "command": "echo hello world",
    "parameters": {},
    "timeout_seconds": 30
  }'
```

### 4. 运行测试脚本
```bash
./test_debug.sh
```

## 验证修复效果

1. **数据一致性**: HTTP创建的任务现在可以被gRPC服务器正确访问
2. **任务分配**: 只有在线且空闲的agent会被分配任务
3. **调试能力**: 通过日志和debug接口可以清楚看到系统状态

## 日志说明

### 任务创建日志
```
Task created: task_1234567890 (0) - Type: test, Command: echo hello world
```

### Agent心跳日志
```
Agent agent_001 (TestAgent) heartbeat - no pending tasks
Agent agent_001 (TestAgent) has pending task available
```

### 任务分配日志
```
Task assigned: Agent agent_001 (TestAgent) got task task_1234567890 (echo hello world)
```

## 注意事项

1. 确保agent定期发送心跳（建议每30秒）
2. 只有状态为 `AgentStatusIdle` 且5分钟内有心跳的agent才会被分配任务
3. 任务状态变化会实时保存到文件系统
4. 重启服务器后数据会从文件系统恢复 