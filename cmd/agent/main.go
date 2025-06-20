package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Axpz/xHub/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AgentClient struct {
	client       proto.AgentServiceClient
	conn         *grpc.ClientConn
	agentID      string
	sessionToken string
	serverAddr   string
	mu           sync.RWMutex
	running      bool
}

func NewAgentClient(serverAddr string) *AgentClient {
	return &AgentClient{
		serverAddr: serverAddr,
		agentID:    generateAgentID(),
	}
}

func (c *AgentClient) Connect() error {
	conn, err := grpc.NewClient(c.serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}

	c.conn = conn
	c.client = proto.NewAgentServiceClient(conn)
	return nil
}

func (c *AgentClient) Register() error {
	hostname, _ := os.Hostname()
	ipAddr := getLocalIP()

	req := &proto.RegisterRequest{
		AgentId:      c.agentID,
		AgentName:    fmt.Sprintf("agent-%s", hostname),
		Version:      "1.0.0",
		Capabilities: map[string]string{"os": "linux", "arch": "amd64"},
		Hostname:     hostname,
		IpAddress:    ipAddr,
	}

	resp, err := c.client.Register(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to register: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("registration failed: %s", resp.Message)
	}

	c.sessionToken = resp.SessionToken
	log.Printf("Agent registered successfully with ID: %s", c.agentID)
	return nil
}

func (c *AgentClient) StartHeartbeat() {
	c.mu.Lock()
	c.running = true
	c.mu.Unlock()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		c.mu.RLock()
		if !c.running {
			c.mu.RUnlock()
			break
		}
		c.mu.RUnlock()

		if err := c.sendHeartbeat(); err != nil {
			log.Printf("Heartbeat failed: %v", err)
		}

		<-ticker.C
	}
}

func (c *AgentClient) sendHeartbeat() error {
	req := &proto.HeartbeatRequest{
		AgentId:      c.agentID,
		SessionToken: c.sessionToken,
		Status:       proto.AgentStatus_IDLE,
		Metrics: map[string]string{
			"cpu_usage": "10%",
			"memory":    "512MB",
		},
	}

	resp, err := c.client.Heartbeat(context.Background(), req)
	if err != nil {
		return fmt.Errorf("heartbeat failed: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("heartbeat failed: %s", resp.Message)
	}

	log.Printf("Heartbeat: %+v", resp)

	if resp.HasTask {
		go c.processTasks()
	}

	return nil
}

func (c *AgentClient) processTasks() {
	for {
		task, err := c.getTask()
		if err != nil {
			log.Printf("Failed to get task: %v", err)
			break
		}

		log.Printf("Task: %+v", task)

		if task == nil {
			break // 没有待处理的任务
		}

		result := c.executeTask(task)
		if err := c.submitTaskResult(task.TaskId, result); err != nil {
			log.Printf("Failed to submit task result: %v", err)
		}
	}
}

func (c *AgentClient) getTask() (*proto.Task, error) {
	req := &proto.GetTaskRequest{
		AgentId:      c.agentID,
		SessionToken: c.sessionToken,
	}

	resp, err := c.client.GetTask(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("get task failed: %v", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("get task failed: %s", resp.Message)
	}

	return resp.Task, nil
}

func (c *AgentClient) executeTask(task *proto.Task) *proto.TaskResult {
	log.Printf("Executing task: %s - %s", task.TaskId, task.Command)

	startTime := time.Now()

	// 从任务参数中获取必要的 iptables 参数
	dport := task.Parameters["dport"]                           // 外部端口
	toDestinationIP := task.Parameters["to-destination-ip"]     // 内部目标IP
	toDestinationPort := task.Parameters["to-destination-port"] // 内部目标端口
	externalInterface := "eth0"                                 // 假设外部接口为 eth0，根据实际情况可能需要从参数获取或配置

	// 检查关键参数是否为空
	if dport == "" || toDestinationIP == "" || toDestinationPort == "" {
		return &proto.TaskResult{
			Success:         false,
			Output:          "",
			Error:           "Missing required iptables parameters: dport, to-destination-ip, or to-destination-port",
			ExecutionTimeMs: time.Since(startTime).Milliseconds(),
			Metadata: map[string]string{
				"executed_at": time.Now().Format(time.RFC3339),
			},
		}
	}

	// 构建 iptables 命令列表
	// 按照之前讨论的，这里包含：
	// 1. DNAT 规则 (PREROUTING)
	// 2. MASQUERADE 规则 (POSTROUTING) - 修正为不带 dport 的通用规则
	// 3. FORWARD 规则 (filter表) - 允许转发到内部IP，并允许内部IP的返回流量
	commands := []string{
		// 启用 IP 转发 (如果尚未启用)
		// "sysctl -w net.ipv4.ip_forward=1", // 运行时生效，但重启可能失效。建议也在 /etc/sysctl.conf 中配置。

		// DNAT 规则
		fmt.Sprintf("iptables -t nat -A PREROUTING -p tcp --dport %s -j DNAT --to-destination %s:%s", dport, toDestinationIP, toDestinationPort),
		fmt.Sprintf("iptables -t nat -A PREROUTING -p udp --dport %s -j DNAT --to-destination %s:%s", dport, toDestinationIP, toDestinationPort),

		// MASQUERADE 规则 (修正版，不带 --dport，并指定出站接口)
		fmt.Sprintf("iptables -t nat -A POSTROUTING -o %s -j MASQUERADE", externalInterface),

		// FORWARD 规则 (Filter 表)
		// 允许新的/已建立的/相关连接从外部到内部目标的转发
		fmt.Sprintf("iptables -A FORWARD -o %s -p tcp -d %s --dport %s -m state --state NEW,ESTABLISHED,RELATED -j ACCEPT", externalInterface, toDestinationIP, toDestinationPort),
		fmt.Sprintf("iptables -A FORWARD -o %s -p udp -d %s --dport %s -m state --state NEW,ESTABLISHED,RELATED -j ACCEPT", externalInterface, toDestinationIP, toDestinationPort),
		// 允许已建立的/相关连接从内部目标返回外部
		fmt.Sprintf("iptables -A FORWARD -i %s -m state --state ESTABLISHED,RELATED -j ACCEPT", externalInterface),
	}

	var executionErrors []string
	for _, command := range commands {
		log.Printf("Executing command: %s", command)
		output, err := exec.Command("bash", "-c", command).CombinedOutput()
		if err != nil {
			errMsg := fmt.Sprintf("Failed to execute command '%s': %v, Output: %s", command, err, string(output))
			log.Printf(errMsg)
			executionErrors = append(executionErrors, errMsg)
		} else {
			log.Printf("Command output: %s", string(output))
		}
	}

	executionTime := time.Since(startTime).Milliseconds()

	if len(executionErrors) > 0 {
		return &proto.TaskResult{
			Success:         false,
			Output:          "Some iptables commands failed to execute.",
			Error:           strings.Join(executionErrors, "\n"),
			ExecutionTimeMs: executionTime,
			Metadata: map[string]string{
				"executed_at": time.Now().Format(time.RFC3339),
			},
		}
	}

	return &proto.TaskResult{
		Success:         true,
		Output:          fmt.Sprintf("Task %s executed successfully. iptables rules applied.", task.TaskId),
		Error:           "",
		ExecutionTimeMs: executionTime,
		Metadata: map[string]string{
			"executed_at": time.Now().Format(time.RFC3339),
		},
	}
}

func (c *AgentClient) submitTaskResult(taskID string, result *proto.TaskResult) error {
	req := &proto.SubmitTaskResultRequest{
		AgentId: c.agentID,
		TaskId:  taskID,
		Result:  result,
	}

	resp, err := c.client.SubmitTaskResult(context.Background(), req)
	if err != nil {
		return fmt.Errorf("submit task result failed: %v", err)
	}

	if !resp.Success {
		return fmt.Errorf("submit task result failed: %s", resp.Message)
	}

	log.Printf("Task result submitted successfully: %s", taskID)
	return nil
}

func (c *AgentClient) Stop() {
	c.mu.Lock()
	c.running = false
	c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
	}
}

func generateAgentID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%s", hostname, getLocalIP())
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func main() {
	serverAddr := "74.121.149.207:20081"
	if len(os.Args) > 1 {
		serverAddr = os.Args[1]
	}

	client := NewAgentClient(serverAddr)

	// 连接到服务器
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect to server: %v", err)
	}
	defer client.Stop()

	// 注册Agent
	if err := client.Register(); err != nil {
		log.Fatalf("Failed to register agent: %v", err)
	}

	// 启动心跳
	go client.StartHeartbeat()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Agent shutting down...")
}
