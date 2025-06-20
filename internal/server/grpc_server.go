package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/Axpz/xHub/internal/models"
	"github.com/Axpz/xHub/internal/store"
	pb "github.com/Axpz/xHub/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	pb.UnimplementedAgentServiceServer
	store   *store.AgentStore
	mu      sync.RWMutex
	grpcSrv *grpc.Server
}

func NewGRPCServer() *GRPCServer {
	return &GRPCServer{
		store: store.NewAgentStore(),
	}
}

// NewGRPCServerWithStore 创建使用指定store的gRPC服务器
func NewGRPCServerWithStore(store *store.AgentStore) *GRPCServer {
	return &GRPCServer{
		store: store,
	}
}

func (s *GRPCServer) Start(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s.grpcSrv = grpc.NewServer()
	pb.RegisterAgentServiceServer(s.grpcSrv, s)

	log.Printf("gRPC server starting on port %d", port)
	return s.grpcSrv.Serve(lis)
}

func (s *GRPCServer) Stop() {
	if s.grpcSrv != nil {
		s.grpcSrv.GracefulStop()
	}
}

// Register 实现Agent注册
func (s *GRPCServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent := &models.Agent{
		ID:           req.AgentId,
		Name:         req.AgentName,
		Version:      req.Version,
		Capabilities: req.Capabilities,
		Hostname:     req.Hostname,
		IPAddress:    req.IpAddress,
		Status:       models.AgentStatusIdle,
		LastSeen:     time.Now(),
		SessionToken: generateSessionToken(),
	}

	s.store.SaveAgent(agent)

	log.Printf("Agent registered: %s (%s)", agent.Name, agent.ID)

	return &pb.RegisterResponse{
		Success:      true,
		Message:      "Agent registered successfully",
		SessionToken: agent.SessionToken,
	}, nil
}

// Heartbeat 实现心跳检测
func (s *GRPCServer) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, exists := s.store.GetAgent(req.AgentId)
	if !exists {
		log.Printf("Heartbeat from unknown agent: %s", req.AgentId)
		return nil, status.Errorf(codes.NotFound, "Agent not found")
	}

	if agent.SessionToken != req.SessionToken {
		log.Printf("Invalid session token from agent: %s", req.AgentId)
		return nil, status.Errorf(codes.Unauthenticated, "Invalid session token")
	}

	// 更新Agent状态
	agent.LastSeen = time.Now()
	agent.Status = models.AgentStatus(req.Status)
	agent.Metrics = req.Metrics

	s.store.SaveAgent(agent)

	// 检查是否有待处理的任务
	hasTask := s.hasPendingTask(req.AgentId)

	// 增加详细的调试日志
	if hasTask {
		log.Printf("Agent %s (%s) has pending task available", req.AgentId, agent.Name)
	}

	return &pb.HeartbeatResponse{
		Success: true,
		Message: "Heartbeat received",
		HasTask: hasTask,
	}, nil
}

// GetTask 获取任务
func (s *GRPCServer) GetTask(ctx context.Context, req *pb.GetTaskRequest) (*pb.GetTaskResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, exists := s.store.GetAgent(req.AgentId)
	if !exists {
		log.Printf("GetTask request from unknown agent: %s", req.AgentId)
		return nil, status.Errorf(codes.NotFound, "Agent not found")
	}

	if agent.SessionToken != req.SessionToken {
		log.Printf("Invalid session token in GetTask from agent: %s", req.AgentId)
		return nil, status.Errorf(codes.Unauthenticated, "Invalid session token")
	}

	// 查找待处理的任务
	task := s.findPendingTask(req.AgentId)
	if task == nil {
		log.Printf("Agent %s (%s) requested task but none available", req.AgentId, agent.Name)
		return &pb.GetTaskResponse{
			Success: true,
			Message: "No pending tasks",
		}, nil
	}

	// 更新任务状态
	task.Status = models.TaskStatusRunning
	task.AssignedTo = req.AgentId
	task.AssignedAt = time.Now()

	s.store.SaveTask(task)

	log.Printf("Task assigned: Agent %s (%s) got task %s (%s)",
		req.AgentId, agent.Name, task.ID, task.Command)

	return &pb.GetTaskResponse{
		Success: true,
		Message: "Task assigned",
		Task: &pb.Task{
			TaskId:         task.ID,
			TaskType:       task.Type,
			Command:        task.Command,
			Parameters:     task.Parameters,
			TimeoutSeconds: task.TimeoutSeconds,
			CreatedAt:      task.CreatedAt.Unix(),
		},
	}, nil
}

// SubmitTaskResult 提交任务结果
func (s *GRPCServer) SubmitTaskResult(ctx context.Context, req *pb.SubmitTaskResultRequest) (*pb.SubmitTaskResultResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, exists := s.store.GetAgent(req.AgentId)
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Agent not found")
	}

	task, exists := s.store.GetTask(req.TaskId)
	if !exists {
		return nil, status.Errorf(codes.NotFound, "Task not found")
	}

	// 更新任务结果
	task.Status = models.TaskStatusCompleted
	task.Result = &models.TaskResult{
		Success:       req.Result.Success,
		Output:        req.Result.Output,
		Error:         req.Result.Error,
		ExecutionTime: req.Result.ExecutionTimeMs,
		Metadata:      req.Result.Metadata,
		CompletedAt:   time.Now(),
	}

	s.store.SaveTask(task)

	log.Printf("Task %s completed by agent %s", task.ID, agent.Name)

	return &pb.SubmitTaskResultResponse{
		Success: true,
		Message: "Task result submitted successfully",
	}, nil
}

// StreamTask 流式任务处理
func (s *GRPCServer) StreamTask(stream pb.AgentService_StreamTaskServer) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}

		switch m := msg.Message.(type) {
		case *pb.TaskStream_TaskRequest:
			// 处理任务请求
			response := s.handleStreamTaskRequest(m.TaskRequest)
			if err := stream.Send(&pb.TaskStream{
				Message: &pb.TaskStream_TaskResponse{
					TaskResponse: response,
				},
			}); err != nil {
				return err
			}
		case *pb.TaskStream_TaskResponse:
			// 处理任务响应
			s.handleStreamTaskResponse(m.TaskResponse)
		}
	}
}

func (s *GRPCServer) handleStreamTaskRequest(req *pb.TaskRequest) *pb.TaskResponse {
	// 这里可以实现流式任务处理逻辑
	return &pb.TaskResponse{
		TaskId:  req.TaskId,
		Success: true,
		Output:  "Stream task processed",
	}
}

func (s *GRPCServer) handleStreamTaskResponse(resp *pb.TaskResponse) {
	// 处理流式任务响应
	log.Printf("Stream task response: %s - %s", resp.TaskId, resp.Output)
}

func (s *GRPCServer) hasPendingTask(agentID string) bool {
	agent, exists := s.store.GetAgent(agentID)
	if !exists {
		return false
	}

	// 检查agent是否在线且空闲
	if time.Since(agent.LastSeen) > 5*time.Minute || agent.Status != models.AgentStatusIdle {
		return false
	}

	for _, task := range s.store.GetPendingTasks() {
		if task.Status == models.TaskStatusPending &&
			(task.AssignedTo == "" || task.AssignedTo == agentID) {
			return true
		}
	}
	return false
}

func (s *GRPCServer) findPendingTask(agentID string) *models.Task {
	agent, exists := s.store.GetAgent(agentID)
	if !exists {
		return nil
	}

	// 检查agent是否在线且空闲
	if time.Since(agent.LastSeen) > 5*time.Minute || agent.Status != models.AgentStatusIdle {
		return nil
	}

	// 优先分配专门分配给该agent的任务
	for _, task := range s.store.GetPendingTasks() {
		if task.Status == models.TaskStatusPending && task.AssignedTo == agentID {
			return task
		}
	}

	// 然后分配未分配的任务
	for _, task := range s.store.GetPendingTasks() {
		if task.Status == models.TaskStatusPending && task.AssignedTo == "" {
			return task
		}
	}
	return nil
}

func generateSessionToken() string {
	return fmt.Sprintf("session_%d", time.Now().UnixNano())
}
