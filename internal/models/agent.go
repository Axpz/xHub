package models

import (
	"time"
)

// AgentStatus 表示Agent的状态
type AgentStatus int

const (
	AgentStatusUnknown AgentStatus = iota
	AgentStatusIdle
	AgentStatusBusy
	AgentStatusOffline
)

// Agent 表示一个Agent实例
type Agent struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Capabilities map[string]string `json:"capabilities"`
	Hostname     string            `json:"hostname"`
	IPAddress    string            `json:"ip_address"`
	Status       AgentStatus       `json:"status"`
	LastSeen     time.Time         `json:"last_seen"`
	SessionToken string            `json:"session_token"`
	Metrics      map[string]string `json:"metrics"`
}

// TaskStatus 表示任务的状态
type TaskStatus int

const (
	TaskStatusPending TaskStatus = iota
	TaskStatusRunning
	TaskStatusCompleted
	TaskStatusFailed
)

// Task 表示一个任务
type Task struct {
	ID             string            `json:"id"`
	Type           string            `json:"type"`
	Command        string            `json:"command"`
	Parameters     map[string]string `json:"parameters"`
	TimeoutSeconds int64             `json:"timeout_seconds"`
	Status         TaskStatus        `json:"status"`
	AssignedTo     string            `json:"assigned_to"`
	AssignedAt     time.Time         `json:"assigned_at"`
	CreatedAt      time.Time         `json:"created_at"`
	Result         *TaskResult       `json:"result,omitempty"`
}

// TaskResult 表示任务执行结果
type TaskResult struct {
	Success       bool              `json:"success"`
	Output        string            `json:"output"`
	Error         string            `json:"error"`
	ExecutionTime int64             `json:"execution_time_ms"`
	Metadata      map[string]string `json:"metadata"`
	CompletedAt   time.Time         `json:"completed_at"`
}
