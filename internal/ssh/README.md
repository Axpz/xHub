# SSH模块

这个模块提供了通过SSH远程安装和管理Agent的功能。

## 功能特性

- SSH连接管理
- 文件上传（SCP）
- 远程Agent安装和启动
- 完整的测试覆盖

## 使用方法

### 基本用法

```go
package main

import (
    "fmt"
    "log"
    "github.com/Axpz/xHub/internal/ssh"
)

func main() {
    // 创建SSH配置
    cfg := &ssh.SSHConfig{
        Host:     "192.168.1.100",
        Port:     22,
        Username: "root",
        Password: "your_password",
    }

    // 测试连接
    client, err := cfg.GetSSHClient()
    if err != nil {
        log.Fatalf("连接失败: %v", err)
    }
    defer client.Close()

    // 安装Agent
    output, err := cfg.InstallAgent()
    if err != nil {
        log.Fatalf("安装失败: %v", err)
    }
    fmt.Println("安装完成:", output)
}
```

### 文件上传

```go
err := cfg.UploadFile("./local/file", "/remote/path/file")
if err != nil {
    log.Fatalf("上传失败: %v", err)
}
```

## 测试

### 运行测试

1. 设置环境变量：
```bash
export SSH_TEST_HOST=your_test_host
export SSH_TEST_USER=your_test_user
export SSH_TEST_PASS=your_test_password
```

2. 构建Agent：
```bash
make build-agent
```

3. 运行测试：
```bash
# 运行所有测试
make test-ssh

# 运行完整测试（包括InstallAgent）
make test-ssh-full

# 运行基准测试
make bench
```

### 测试脚本

使用提供的测试脚本：

```bash
chmod +x scripts/test-ssh.sh
./scripts/test-ssh.sh
```

## 注意事项

- 确保目标服务器支持SSH和SCP
- 确保有足够的权限执行chmod和nohup命令
- 测试时需要真实的SSH服务器环境
- 生产环境中建议使用SSH密钥而不是密码认证

## API参考

### SSHConfig

```go
type SSHConfig struct {
    Host     string
    Port     int
    Username string
    Password string
}
```

### 方法

- `GetSSHClient() (*ssh.Client, error)` - 获取SSH客户端连接
- `UploadFile(localPath, remotePath string) error` - 上传文件到远程服务器
- `InstallAgent() (output string, err error)` - 安装并启动Agent

## 安全考虑

- 在生产环境中使用SSH密钥认证
- 限制SSH用户权限
- 使用防火墙限制SSH访问
- 定期更新SSH服务 