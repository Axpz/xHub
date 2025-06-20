package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
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

	// 这里实现具体的任务执行逻辑
	// 目前只是模拟执行
	time.Sleep(2 * time.Second)

	dport := task.Parameters["dport"]
	to_destination_ip := task.Parameters["to-destination-ip"]
	to_destination_port := task.Parameters["to-destination-port"]

	commands := []string{
		fmt.Sprintf("iptables -t nat -A PREROUTING -p tcp --dport %s -j DNAT --to-destination %s:%s", dport, to_destination_ip, to_destination_port),
		fmt.Sprintf("iptables -t nat -A PREROUTING -p udp --dport %s -j DNAT --to-destination %s:%s", dport, to_destination_ip, to_destination_port),
		fmt.Sprintf("iptables -t nat -A POSTROUTING -d %s -p tcp --dport %s -j MASQUERADE", to_destination_ip, dport),
		fmt.Sprintf("iptables -t nat -A POSTROUTING -d %s -p udp --dport %s -j MASQUERADE", to_destination_ip, dport),
	}

	for _, command := range commands {
		log.Printf("Executing command: %s", command)
		if dport != "" && to_destination_ip != "" && to_destination_port != "" {
			output, err := exec.Command("bash", "-c", command).CombinedOutput()
			if err != nil {
				log.Printf("Failed to execute command: %v", err)
			}
			log.Printf("Command output: %s", string(output))
		}
	}

	executionTime := time.Since(startTime).Milliseconds()

	return &proto.TaskResult{
		Success:         true,
		Output:          fmt.Sprintf("Task %s executed successfully", task.TaskId),
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
	serverAddr := "localhost:20081"
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
