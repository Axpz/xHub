package server

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/Axpz/xHub/internal/models"
	"github.com/Axpz/xHub/internal/ssh"
	"github.com/Axpz/xHub/internal/store"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type HTTPServer struct {
	store  *store.AgentStore
	router *gin.Engine
}

func NewHTTPServer(store *store.AgentStore) *HTTPServer {
	server := &HTTPServer{
		store:  store,
		router: gin.Default(),
	}

	server.setupRoutes()
	return server
}

func (s *HTTPServer) setupRoutes() {
	// 添加CORS中间件
	s.router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API路由组
	api := s.router.Group("/api/v1")
	{
		// Agent相关接口
		agents := api.Group("/agents")
		{
			agents.GET("", s.listAgents)
			agents.POST("", s.installAgent)
			agents.GET("/:id", s.getAgent)
			agents.DELETE("/:id", s.deleteAgent)
		}

		// Task相关接口
		tasks := api.Group("/tasks")
		{
			tasks.GET("", s.listTasks)
			tasks.POST("", s.createTask)
			tasks.GET("/:id", s.getTask)
			tasks.DELETE("/:id", s.deleteTask)
		}

		// 系统状态接口
		api.GET("/status", s.getStatus)

		// 调试接口
		api.GET("/debug", s.debugInfo)
	}

	// 健康检查
	s.router.GET("/health", s.healthCheck)
}

func (s *HTTPServer) Start(port int) error {
	log.Printf("HTTP server starting on port %d", port)
	return s.router.Run(":" + strconv.Itoa(port))
}

// listAgents 获取所有Agent列表
func (s *HTTPServer) listAgents(c *gin.Context) {
	agents := s.store.GetAllAgents()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    agents,
		"count":   len(agents),
	})
}

// add a server which should install agent
func (s *HTTPServer) installAgent(c *gin.Context) {
	var req struct {
		Host     string `json:"host" binding:"required"`
		Port     int    `json:"port" binding:"required"`
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	log.Printf("Installing agent to %s:%d with user %s", req.Host, req.Port, req.Username)

	sshConfig := &ssh.SSHConfig{
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
	}

	// 使用goroutine和channel来处理超时
	done := make(chan struct {
		output string
		err    error
	}, 1)

	go func() {
		output, err := sshConfig.InstallAgent()
		done <- struct {
			output string
			err    error
		}{output, err}
	}()

	// 设置超时时间
	select {
	case result := <-done:
		if result.err != nil {
			log.Printf("Agent installation failed: %v", result.err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to install agent: " + result.err.Error(),
			})
			return
		}
		log.Printf("Agent installation completed successfully")
		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"message": result.output,
		})
	case <-time.After(300 * time.Second): // 300秒超时
		log.Printf("Agent installation timed out")
		c.JSON(http.StatusRequestTimeout, gin.H{
			"success": false,
			"message": "Agent installation timed out after 5 minutes",
		})
	}
}

// getAgent 获取单个Agent信息
func (s *HTTPServer) getAgent(c *gin.Context) {
	id := c.Param("id")

	agent, exists := s.store.GetAgent(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Agent not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    agent,
	})
}

// deleteAgent 删除Agent
func (s *HTTPServer) deleteAgent(c *gin.Context) {
	id := c.Param("id")

	if err := s.store.DeleteAgent(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete agent: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Agent deleted successfully",
	})
}

// listTasks 获取所有Task列表
func (s *HTTPServer) listTasks(c *gin.Context) {
	status := c.Query("status")

	var tasks []*models.Task
	if status == "pending" {
		tasks = s.store.GetPendingTasks()
	} else {
		tasks = s.store.GetAllTasks()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    tasks,
		"count":   len(tasks),
	})
}

// createTask 创建新任务
func (s *HTTPServer) createTask(c *gin.Context) {
	var req struct {
		Type           string            `json:"type" binding:"required"`
		Command        string            `json:"command" binding:"required"`
		Parameters     map[string]string `json:"parameters"`
		TimeoutSeconds int64             `json:"timeout_seconds"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request: " + err.Error(),
		})
		return
	}

	task := &models.Task{
		ID:             generateTaskID(),
		Type:           req.Type,
		Command:        req.Command,
		Parameters:     req.Parameters,
		TimeoutSeconds: req.TimeoutSeconds,
		Status:         models.TaskStatusPending,
		CreatedAt:      time.Now(),
	}

	if err := s.store.SaveTask(task); err != nil {
		log.Printf("Failed to save task %s: %v", task.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create task: " + err.Error(),
		})
		return
	}

	log.Printf("Task created: %s (%d) - Type: %s, Command: %s",
		task.ID, task.Status, task.Type, task.Command)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    task,
		"message": "Task created successfully",
	})
}

// getTask 获取单个Task信息
func (s *HTTPServer) getTask(c *gin.Context) {
	id := c.Param("id")

	task, exists := s.store.GetTask(id)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Task not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    task,
	})
}

// deleteTask 删除Task
func (s *HTTPServer) deleteTask(c *gin.Context) {
	id := c.Param("id")

	if err := s.store.DeleteTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete task: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Task deleted successfully",
	})
}

// getStatus 获取系统状态
func (s *HTTPServer) getStatus(c *gin.Context) {
	agents := s.store.GetAllAgents()
	tasks := s.store.GetAllTasks()
	pendingTasks := s.store.GetPendingTasks()

	// 统计在线Agent数量
	onlineAgents := 0
	for _, agent := range agents {
		if time.Since(agent.LastSeen) < 5*time.Minute {
			onlineAgents++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_agents":  len(agents),
			"online_agents": onlineAgents,
			"total_tasks":   len(tasks),
			"pending_tasks": len(pendingTasks),
			"server_time":   time.Now().Unix(),
		},
	})
}

// healthCheck 健康检查
func (s *HTTPServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "Service is healthy",
		"time":    time.Now().Unix(),
	})
}

// debugInfo 调试信息
func (s *HTTPServer) debugInfo(c *gin.Context) {
	agents := s.store.GetAllAgents()
	tasks := s.store.GetAllTasks()
	pendingTasks := s.store.GetPendingTasks()

	// 统计在线Agent数量
	onlineAgents := 0
	idleAgents := 0
	for _, agent := range agents {
		if time.Since(agent.LastSeen) < 5*time.Minute {
			onlineAgents++
			if agent.Status == models.AgentStatusIdle {
				idleAgents++
			}
		}
	}

	// 统计任务状态
	taskStats := make(map[string]int)
	for _, task := range tasks {
		status := "unknown"
		switch task.Status {
		case models.TaskStatusPending:
			status = "pending"
		case models.TaskStatusRunning:
			status = "running"
		case models.TaskStatusCompleted:
			status = "completed"
		case models.TaskStatusFailed:
			status = "failed"
		}
		taskStats[status]++
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"agents": gin.H{
				"total":   len(agents),
				"online":  onlineAgents,
				"idle":    idleAgents,
				"details": agents,
			},
			"tasks": gin.H{
				"total":   len(tasks),
				"pending": len(pendingTasks),
				"stats":   taskStats,
				"details": tasks,
			},
			"server_time": time.Now().Unix(),
		},
	})
}

func generateTaskID() string {
	return "task_" + strconv.FormatInt(time.Now().UnixNano(), 10)
}
