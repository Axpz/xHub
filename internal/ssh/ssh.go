package ssh

import (
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHConfig holds SSH connection info
type SSHConfig struct {
	Host     string
	Port     int
	Username string
	Password string
}

func (cfg *SSHConfig) GetSSHClient() (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User: cfg.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(cfg.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), sshConfig)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %w", err)
	}
	return client, nil
}

// UploadFile 上传本地文件到远程路径
func (cfg *SSHConfig) UploadFile(localPath, remotePath string) error {
	client, err := cfg.GetSSHClient()
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 session 失败: %w", err)
	}
	defer session.Close()

	// 打开本地文件
	srcFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %w", err)
	}
	defer srcFile.Close()

	fileInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 创建目标管道：scp -t <remotePath>（目标模式）
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		fmt.Fprintf(w, "C0755 %d %s\n", fileInfo.Size(), path.Base(remotePath))
		io.Copy(w, srcFile)
		fmt.Fprint(w, "\x00")
	}()

	// 启动 scp 接收模式
	if err := session.Run(fmt.Sprintf("scp -t %s", path.Dir(remotePath))); err != nil {
		return fmt.Errorf("远程执行 scp 失败: %w", err)
	}

	return nil
}

// InstallAgent 安装并启动 agent
func (cfg *SSHConfig) InstallAgent() (output string, err error) {
	localAgentPath := "./bin/xhub-agent"
	remoteAgentPath := "/root/xhub-agent"

	// 1. 上传 agent 可执行文件
	if err := cfg.UploadFile(localAgentPath, remoteAgentPath); err != nil {
		return "", fmt.Errorf("上传 agent 失败: %w", err)
	}

	// 2. 连接远程执行命令
	client, err := cfg.GetSSHClient()
	if err != nil {
		return "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建远程 session 失败: %w", err)
	}
	defer session.Close()

	// 3. 运行启动命令（chmod + nohup）
	cmd := fmt.Sprintf("chmod +x %s && nohup %s > ./xhub-agent.log 2>&1 &", remoteAgentPath, remoteAgentPath)
	outputBytes, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(outputBytes), fmt.Errorf("远程启动 agent 失败: %w", err)
	}

	return string(outputBytes), nil
}

// UploadAndStartAgent 上传并启动agent（无输出版本）
func (cfg *SSHConfig) UploadAndStartAgent() (string, error) {
	return cfg.InstallAgent()
}
