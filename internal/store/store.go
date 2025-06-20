package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/Axpz/xHub/internal/models"
)

// AgentStore 提供Agent和Task的存储功能
type AgentStore struct {
	mu      sync.RWMutex
	dataDir string
	agents  map[string]*models.Agent
	tasks   map[string]*models.Task
}

// NewAgentStore 创建新的AgentStore实例
func NewAgentStore() *AgentStore {
	dataDir := "./data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		panic(fmt.Sprintf("failed to create data directory: %v", err))
	}

	store := &AgentStore{
		dataDir: dataDir,
		agents:  make(map[string]*models.Agent),
		tasks:   make(map[string]*models.Task),
	}

	// 加载现有数据
	store.loadData()

	return store
}

// SaveAgent 保存Agent信息
func (s *AgentStore) SaveAgent(agent *models.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.agents[agent.ID] = agent
	return s.saveAgents()
}

// GetAgent 获取Agent信息
func (s *AgentStore) GetAgent(id string) (*models.Agent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agent, exists := s.agents[id]
	return agent, exists
}

// GetAllAgents 获取所有Agent
func (s *AgentStore) GetAllAgents() []*models.Agent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]*models.Agent, 0, len(s.agents))
	for _, agent := range s.agents {
		agents = append(agents, agent)
	}
	return agents
}

// DeleteAgent 删除Agent
func (s *AgentStore) DeleteAgent(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.agents, id)
	return s.saveAgents()
}

// SaveTask 保存Task信息
func (s *AgentStore) SaveTask(task *models.Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tasks[task.ID] = task

	// 添加日志输出
	fmt.Printf("Saving task %s (status: %d, assigned_to: %s)\n",
		task.ID, task.Status, task.AssignedTo)

	return s.saveTasks()
}

// GetTask 获取Task信息
func (s *AgentStore) GetTask(id string) (*models.Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, exists := s.tasks[id]
	return task, exists
}

// GetAllTasks 获取所有Task
func (s *AgentStore) GetAllTasks() []*models.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tasks := make([]*models.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetPendingTasks 获取待处理的任务
func (s *AgentStore) GetPendingTasks() []*models.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pendingTasks []*models.Task
	for _, task := range s.tasks {
		if task.Status == models.TaskStatusPending {
			pendingTasks = append(pendingTasks, task)
		}
	}
	return pendingTasks
}

// DeleteTask 删除Task
func (s *AgentStore) DeleteTask(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tasks, id)
	return s.saveTasks()
}

// loadData 加载数据
func (s *AgentStore) loadData() {
	s.loadAgents()
	s.loadTasks()
}

// loadAgents 加载Agent数据
func (s *AgentStore) loadAgents() {
	filePath := filepath.Join(s.dataDir, "agents.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Error reading agents file: %v\n", err)
		}
		return
	}

	var agents map[string]*models.Agent
	if err := json.Unmarshal(data, &agents); err != nil {
		fmt.Printf("Error unmarshaling agents: %v\n", err)
		return
	}

	s.mu.Lock()
	s.agents = agents
	s.mu.Unlock()
}

// loadTasks 加载Task数据
func (s *AgentStore) loadTasks() {
	filePath := filepath.Join(s.dataDir, "tasks.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Printf("Error reading tasks file: %v\n", err)
		}
		return
	}

	var tasks map[string]*models.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		fmt.Printf("Error unmarshaling tasks: %v\n", err)
		return
	}

	s.mu.Lock()
	s.tasks = tasks
	s.mu.Unlock()
}

// saveAgents 保存Agent数据
func (s *AgentStore) saveAgents() error {
	filePath := filepath.Join(s.dataDir, "agents.json")
	data, err := json.MarshalIndent(s.agents, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal agents: %v", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// saveTasks 保存Task数据
func (s *AgentStore) saveTasks() error {
	filePath := filepath.Join(s.dataDir, "tasks.json")
	data, err := json.MarshalIndent(s.tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %v", err)
	}

	return os.WriteFile(filePath, data, 0644)
}
