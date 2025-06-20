package ssh

import (
	"fmt"
	"io"
	"log"
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
		Timeout:         30 * time.Second,
	}

	log.Printf("Connecting to SSH server %s:%d as user %s", cfg.Host, cfg.Port, cfg.Username)
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH连接失败: %w", err)
	}

	log.Printf("SSH connection established successfully")
	return client, nil
}

// UploadFile 上传本地文件到远程路径
func (cfg *SSHConfig) UploadFile(localPath, remotePath string) error {
	log.Printf("Uploading file from %s to %s", localPath, remotePath)

	// 检查本地文件是否存在
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return fmt.Errorf("本地文件不存在: %s", localPath)
	}

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

	log.Printf("File size: %d bytes", fileInfo.Size())

	// 创建目标管道：scp -t <remotePath>（目标模式）
	go func() {
		w, _ := session.StdinPipe()
		defer w.Close()
		fmt.Fprintf(w, "C0755 %d %s\n", fileInfo.Size(), path.Base(remotePath))
		io.Copy(w, srcFile)
		fmt.Fprint(w, "\x00")
	}()

	// 启动 scp 接收模式，设置超时
	done := make(chan error, 1)
	go func() {
		done <- session.Run(fmt.Sprintf("scp -t %s", path.Dir(remotePath)))
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("远程执行 scp 失败: %w", err)
		}
	case <-time.After(60 * time.Second): // 60秒超时
		return fmt.Errorf("文件上传超时")
	}

	log.Printf("File uploaded successfully")
	return nil
}

// InstallAgent 安装并启动 agent
func (cfg *SSHConfig) InstallAgent() (output string, err error) {
	localAgentPath := "./bin/xhub-agent"
	remoteAgentPath := "/root/xhub-agent"

	log.Printf("=== Starting agent installation to %s:%d ===", cfg.Host, cfg.Port)

	// 获取SSH客户端连接
	client, err := cfg.GetSSHClient()
	if err != nil {
		return "", fmt.Errorf("SSH连接失败: %w", err)
	}
	defer client.Close()

	// 步骤1: 检查并杀掉正在运行的xhub-agent进程
	log.Printf("Step 1: Checking and killing existing xhub-agent processes...")
	if err := cfg.killExistingAgent(client); err != nil {
		log.Printf("Warning: Failed to kill existing agent: %v", err)
	}

	// 步骤2: 删除远程服务器上的旧xhub-agent文件
	log.Printf("Step 2: Removing old xhub-agent file...")
	if err := cfg.removeOldAgent(client, remoteAgentPath); err != nil {
		log.Printf("Warning: Failed to remove old agent file: %v", err)
	}

	// 步骤3: 上传新的agent文件
	log.Printf("Step 3: Uploading new agent file...")
	if err := cfg.UploadFile(localAgentPath, remoteAgentPath); err != nil {
		return "", fmt.Errorf("上传agent失败: %w", err)
	}

	// 步骤4: 设置文件权限
	log.Printf("Step 4: Setting file permissions...")
	if err := cfg.setFilePermissions(client, remoteAgentPath); err != nil {
		return "", fmt.Errorf("设置文件权限失败: %w", err)
	}

	// 步骤5: 启动agent
	log.Printf("Step 5: Starting agent...")
	pid, err := cfg.startAgent(client, remoteAgentPath)
	if err != nil {
		return "", fmt.Errorf("启动agent失败: %w", err)
	}

	// 步骤6: 验证agent是否正在运行
	log.Printf("Step 6: Verifying agent is running...")
	if err := cfg.verifyAgentRunning(client); err != nil {
		return "", fmt.Errorf("验证agent运行失败: %w", err)
	}

	// 步骤7: 检查IP转发是否支持
	log.Printf("Step 7: Checking if IP forwarding is supported...")
	if output, err := cfg.supportIPForward(client); err != nil {
		return output, fmt.Errorf("检查IP转发失败: %w", err)
	}

	log.Printf("=== Agent installation completed successfully ===")
	return fmt.Sprintf("Agent started successfully with PID: %s", pid), nil
}

// killExistingAgent 检查并杀掉正在运行的xhub-agent进程
func (cfg *SSHConfig) killExistingAgent(client *ssh.Client) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建session失败: %w", err)
	}
	defer session.Close()

	// 查找并杀掉xhub-agent进程
	killCmd := "pkill -f xhub-agent || true"
	output, err := session.CombinedOutput(killCmd)
	if err != nil {
		return fmt.Errorf("执行kill命令失败: %w", err)
	}

	log.Printf("Kill command output: %s", string(output))
	return nil
}

// removeOldAgent 删除远程服务器上的旧agent文件
func (cfg *SSHConfig) removeOldAgent(client *ssh.Client, remotePath string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建session失败: %w", err)
	}
	defer session.Close()

	// 删除旧文件
	removeCmd := fmt.Sprintf("rm -f %s", remotePath)
	output, err := session.CombinedOutput(removeCmd)
	if err != nil {
		return fmt.Errorf("删除旧文件失败: %w", err)
	}

	log.Printf("Remove command output: %s", string(output))
	return nil
}

// setFilePermissions 设置agent文件权限
func (cfg *SSHConfig) setFilePermissions(client *ssh.Client, remotePath string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建session失败: %w", err)
	}
	defer session.Close()

	// 设置可执行权限
	chmodCmd := fmt.Sprintf("chmod +x %s", remotePath)
	output, err := session.CombinedOutput(chmodCmd)
	if err != nil {
		return fmt.Errorf("设置文件权限失败: %w", err)
	}

	log.Printf("Chmod command output: %s", string(output))
	return nil
}

// startAgent 启动agent并返回进程ID
func (cfg *SSHConfig) startAgent(client *ssh.Client, remotePath string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建session失败: %w", err)
	}
	defer session.Close()

	// 启动agent并获取PID
	startCmd := fmt.Sprintf("nohup %s > /root/xhub-agent.log 2>&1 & echo $!", remotePath)
	output, err := session.CombinedOutput(startCmd)
	if err != nil {
		return "", fmt.Errorf("启动agent失败: %w", err)
	}

	pid := string(output)
	log.Printf("Agent started with PID: %s", pid)
	return pid, nil
}

// verifyAgentRunning 验证agent是否正在运行
func (cfg *SSHConfig) verifyAgentRunning(client *ssh.Client) error {
	// 等待2秒让进程完全启动
	time.Sleep(2 * time.Second)

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建验证session失败: %w", err)
	}
	defer session.Close()

	// 检查进程是否存在
	checkCmd := "pgrep -f xhub-agent"
	output, err := session.CombinedOutput(checkCmd)
	if err != nil || len(output) == 0 {
		return fmt.Errorf("xhub-agent 未在远程运行: %w", err)
	}

	if len(output) == 0 {
		return fmt.Errorf("agent进程未找到")
	}

	log.Printf("Agent verification successful: %s", string(output))
	return nil
}

func (cfg *SSHConfig) supportIPForward(client *ssh.Client) (string, error) {
	log.Printf("Checking if IP forwarding is supported...")

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建session失败: %w", err)
	}
	defer session.Close()

	echoCmd := "echo 1 > /proc/sys/net/ipv4/ip_forward"
	output, err := session.CombinedOutput(echoCmd)
	if err != nil {
		return "", fmt.Errorf("执行echo命令失败: %w", err)
	}

	log.Printf("Echo command output: %s", string(output))
	return string(output), nil
}
