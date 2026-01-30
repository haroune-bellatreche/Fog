package main

import (
	"context"
	"container/heap"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

const (
	MaxLoadThreshold = 0.8 // Reject tasks if load > 80%
)

// FogNode represents a fog computing node
type FogNode struct {
	ID       string    `json:"id"`
	Location string    `json:"location"`
	Status   string    `json:"status"`
	Load     float64   `json:"load"`
	LastSeen time.Time `json:"last_seen"`
}

// Task represents a computational task
type Task struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Payload     map[string]interface{} `json:"payload"`
	Priority    int                    `json:"priority"`
	Criticality int                    `json:"criticality"` // 1-5, higher = more critical
	EstimatedLatency time.Duration     `json:"estimated_latency,omitempty"`
	CPUCost     float64                `json:"cpu_cost,omitempty"`     // Estimated CPU usage (0.0-1.0)
	RAMCost     float64                `json:"ram_cost,omitempty"`     // Estimated RAM usage (0.0-1.0)
	StorageCost float64                `json:"storage_cost,omitempty"` // Estimated storage usage (MB)
	EnergyCost  float64                `json:"energy_cost,omitempty"`  // Estimated energy consumption (Wh)
	NetworkLatency time.Duration       `json:"network_latency,omitempty"` // Network latency to node
	Status      string                 `json:"status"`
	Result      interface{}            `json:"result,omitempty"`
	SubmittedAt time.Time              `json:"submitted_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// TaskHeap implements a min-heap based on intelligent scheduling score
// Lower score = higher priority for execution
type TaskHeap []*Task

func (h TaskHeap) Len() int           { return len(h) }
func (h TaskHeap) Less(i, j int) bool { 
	scoreI := h[i].calculateScore()
	scoreJ := h[j].calculateScore()
	return scoreI < scoreJ 
}
func (h TaskHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *TaskHeap) Push(x interface{}) {
	*h = append(*h, x.(*Task))
}

func (h *TaskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}

// calculateScore computes intelligent scheduling score
// Lower score = should be executed first
// Considers: priority, criticality, latency, resource usage, energy efficiency
func (t *Task) calculateScore() float64 {
	baseScore := float64(t.Priority)
	criticalityBonus := float64(5 - t.Criticality) * 10 // Higher criticality reduces score
	latencyPenalty := t.EstimatedLatency.Seconds() * 0.1
	networkPenalty := t.NetworkLatency.Seconds() * 0.05

	// Resource efficiency: prefer tasks that use fewer resources
	resourcePenalty := (t.CPUCost + t.RAMCost) * 5
	storagePenalty := t.StorageCost * 0.001

	// Energy efficiency: prefer low energy consumption
	energyPenalty := t.EnergyCost * 2

	return baseScore + criticalityBonus + latencyPenalty + networkPenalty +
		   resourcePenalty + storagePenalty + energyPenalty
}

// FogCompute handles fog computing operations
type FogCompute struct {
	node    FogNode
	tasks   map[string]*Task
	taskHeap TaskHeap
	mu      sync.RWMutex
	cond    *sync.Cond
	metrics Metrics
	// Available resources
	availableCPU    float64
	availableRAM    float64
	availableStorage float64
	energyLevel     float64 // Current energy level (0.0-1.0)
}

// Metrics tracks performance metrics
type Metrics struct {
	TasksProcessed int           `json:"tasks_processed"`
	AvgLatency     time.Duration `json:"avg_latency"`
	CurrentLoad    float64       `json:"current_load"`
	mu             sync.RWMutex
}

// NewFogCompute creates a new fog computing instance
func NewFogCompute(nodeID, location string) *FogCompute {
	fc := &FogCompute{
		node: FogNode{
			ID:       nodeID,
			Location: location,
			Status:   "active",
			Load:     0.0,
			LastSeen: time.Now(),
		},
		tasks:   make(map[string]*Task),
		taskHeap: make(TaskHeap, 0),
		metrics: Metrics{
			TasksProcessed: 0,
			AvgLatency:     0,
			CurrentLoad:    0.0,
		},
		// Initialize available resources
		availableCPU:     1.0,  // 100% CPU available
		availableRAM:     1.0,  // 100% RAM available
		availableStorage: 1000.0, // 1000 MB storage available
		energyLevel:      1.0,  // 100% energy level
	}
	fc.cond = sync.NewCond(&fc.mu)
	heap.Init(&fc.taskHeap)
	return fc
}

// Start begins processing tasks
func (fc *FogCompute) Start(ctx context.Context) {
	log.Println("Starting fog computing node:", fc.node.ID)
	
	// Start worker pool
	numWorkers := 5
	for i := 0; i < numWorkers; i++ {
		go fc.worker(ctx, i)
	}

	// Start metrics updater
	go fc.updateMetrics(ctx)
}

// worker processes tasks from the priority queue
func (fc *FogCompute) worker(ctx context.Context, workerID int) {
	log.Printf("Worker %d started\n", workerID)
	
	for {
		fc.mu.Lock()
		for fc.taskHeap.Len() == 0 {
			fc.cond.Wait() // Wait for tasks to be available
		}
		task := heap.Pop(&fc.taskHeap).(*Task)
		fc.mu.Unlock()
		
		select {
		case <-ctx.Done():
			log.Printf("Worker %d stopping\n", workerID)
			return
		default:
			fc.processTask(task)
		}
	}
}

// processTask executes a single task
func (fc *FogCompute) processTask(task *Task) {
	startTime := time.Now()
	
	fc.mu.Lock()
	task.Status = "processing"
	fc.mu.Unlock()

	log.Printf("Processing task %s of type %s (priority %d, criticality %d, score %.2f)\n", 
		task.ID, task.Type, task.Priority, task.Criticality, task.calculateScore())

	// Simulate different types of fog computing tasks
	var result interface{}

	switch task.Type {
	case "data_aggregation":
		result = fc.aggregateData(task.Payload)
	case "edge_analytics":
		result = fc.performAnalytics(task.Payload)
	case "preprocessing":
		result = fc.preprocessData(task.Payload)
	case "caching":
		result = fc.cacheData(task.Payload)
	default:
		result = map[string]string{"error": "unknown task type"}
	}

	completedAt := time.Now()
	latency := completedAt.Sub(startTime)

	fc.mu.Lock()
	task.Status = "completed"
	task.Result = result
	task.CompletedAt = &completedAt

	// Release resources
	fc.availableCPU += task.CPUCost
	fc.availableRAM += task.RAMCost
	fc.availableStorage += task.StorageCost
	fc.energyLevel += task.EnergyCost

	fc.mu.Unlock()

	// Update metrics
	fc.metrics.mu.Lock()
	fc.metrics.TasksProcessed++
	if fc.metrics.AvgLatency == 0 {
		fc.metrics.AvgLatency = latency
	} else {
		fc.metrics.AvgLatency = (fc.metrics.AvgLatency + latency) / 2
	}
	fc.metrics.mu.Unlock()

	log.Printf("Task %s completed in %v\n", task.ID, latency)
}

// Simulated fog computing operations
func (fc *FogCompute) aggregateData(payload map[string]interface{}) map[string]interface{} {
	time.Sleep(100 * time.Millisecond) // Simulate processing
	return map[string]interface{}{
		"operation": "data_aggregation",
		"status":    "success",
		"summary":   "Aggregated sensor data from multiple sources",
		"count":     42,
	}
}

func (fc *FogCompute) performAnalytics(payload map[string]interface{}) map[string]interface{} {
	time.Sleep(200 * time.Millisecond) // Simulate processing
	return map[string]interface{}{
		"operation": "edge_analytics",
		"status":    "success",
		"insights":  "Anomaly detected in sensor readings",
		"confidence": 0.87,
	}
}

func (fc *FogCompute) preprocessData(payload map[string]interface{}) map[string]interface{} {
	time.Sleep(50 * time.Millisecond) // Simulate processing
	return map[string]interface{}{
		"operation": "preprocessing",
		"status":    "success",
		"filtered":  true,
		"normalized": true,
	}
}

func (fc *FogCompute) cacheData(payload map[string]interface{}) map[string]interface{} {
	time.Sleep(30 * time.Millisecond) // Simulate processing
	return map[string]interface{}{
		"operation": "caching",
		"status":    "success",
		"cached":    true,
		"ttl":       3600,
	}
}

// updateMetrics periodically updates node metrics
func (fc *FogCompute) updateMetrics(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fc.mu.Lock()
			fc.node.Load = float64(fc.taskHeap.Len()) / 100.0
			fc.node.LastSeen = time.Now()
			fc.mu.Unlock()

			fc.metrics.mu.Lock()
			fc.metrics.CurrentLoad = fc.node.Load
			fc.metrics.mu.Unlock()
		}
	}
}

// HTTP Handlers
func (fc *FogCompute) handleSubmitTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Intelligent scheduling: check current load and available resources
	fc.mu.RLock()
	currentLoad := fc.node.Load
	queueSize := fc.taskHeap.Len()
	availableCPU := fc.availableCPU
	availableRAM := fc.availableRAM
	availableStorage := fc.availableStorage
	energyLevel := fc.energyLevel
	fc.mu.RUnlock()

	if currentLoad > MaxLoadThreshold || queueSize > 50 {
		log.Printf("Task rejected: high load (%.2f) or queue size (%d)\n", currentLoad, queueSize)
		http.Error(w, "Node overloaded, task rejected", http.StatusServiceUnavailable)
		return
	}

	// Set defaults for resource costs first
	if task.CPUCost == 0 {
		switch task.Type {
		case "data_aggregation":
			task.CPUCost = 0.2
		case "edge_analytics":
			task.CPUCost = 0.4
		case "preprocessing":
			task.CPUCost = 0.1
		case "caching":
			task.CPUCost = 0.05
		default:
			task.CPUCost = 0.2
		}
	}
	if task.RAMCost == 0 {
		switch task.Type {
		case "data_aggregation":
			task.RAMCost = 0.15
		case "edge_analytics":
			task.RAMCost = 0.3
		case "preprocessing":
			task.RAMCost = 0.1
		case "caching":
			task.RAMCost = 0.05
		default:
			task.RAMCost = 0.15
		}
	}
	if task.StorageCost == 0 {
		switch task.Type {
		case "data_aggregation":
			task.StorageCost = 50.0
		case "edge_analytics":
			task.StorageCost = 100.0
		case "preprocessing":
			task.StorageCost = 25.0
		case "caching":
			task.StorageCost = 10.0
		default:
			task.StorageCost = 50.0
		}
	}
	if task.EnergyCost == 0 {
		task.EnergyCost = task.CPUCost * 0.5
	}
	if task.NetworkLatency == 0 {
		task.NetworkLatency = 10 * time.Millisecond
	}

	// Check resource availability
	if task.CPUCost > availableCPU || task.RAMCost > availableRAM || task.StorageCost > availableStorage {
		log.Printf("Task rejected: insufficient resources (CPU: %.2f/%.2f, RAM: %.2f/%.2f, Storage: %.2f/%.2f)\n",
			task.CPUCost, availableCPU, task.RAMCost, availableRAM, task.StorageCost, availableStorage)
		http.Error(w, "Insufficient resources, task rejected", http.StatusServiceUnavailable)
		return
	}

	// Check energy level for critical tasks
	if task.Criticality >= 4 && energyLevel < 0.3 {
		log.Printf("Critical task rejected: low energy level (%.2f)\n", energyLevel)
		http.Error(w, "Low energy level, critical task rejected", http.StatusServiceUnavailable)
		return
	}

	task.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	task.Status = "queued"
	task.SubmittedAt = time.Now()

	fc.mu.Lock()
	// Reserve resources
	fc.availableCPU -= task.CPUCost
	fc.availableRAM -= task.RAMCost
	fc.availableStorage -= task.StorageCost
	fc.energyLevel -= task.EnergyCost

	fc.tasks[task.ID] = &task
	heap.Push(&fc.taskHeap, &task)
	fc.cond.Signal() // Wake up one waiting worker
	fc.mu.Unlock()

	log.Printf("Task %s submitted: type=%s, priority=%d, criticality=%d, estimated_latency=%v, resources_reserved: CPU=%.2f, RAM=%.2f, Storage=%.2f, Energy=%.2f\n", 
		task.ID, task.Type, task.Priority, task.Criticality, task.EstimatedLatency, task.CPUCost, task.RAMCost, task.StorageCost, task.EnergyCost)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (fc *FogCompute) handleGetTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	fc.mu.RLock()
	task, exists := fc.tasks[taskID]
	fc.mu.RUnlock()

	if !exists {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (fc *FogCompute) handleGetStatus(w http.ResponseWriter, r *http.Request) {
	fc.mu.RLock()
	node := fc.node
	fc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

func (fc *FogCompute) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	fc.metrics.mu.RLock()
	metrics := fc.metrics
	fc.metrics.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks_processed": metrics.TasksProcessed,
		"avg_latency_ms":  metrics.AvgLatency.Milliseconds(),
		"current_load":    metrics.CurrentLoad,
	})
}

func (fc *FogCompute) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"node":   fc.node.ID,
	})
}

func main() {
	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		nodeID = "fog-node-1"
	}

	location := os.Getenv("LOCATION")
	if location == "" {
		location = "edge-site-1"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fc := NewFogCompute(nodeID, location)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fc.Start(ctx)

	// Setup HTTP routes
	r := mux.NewRouter()

	// CORS middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			
			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			
			next.ServeHTTP(w, r)
		})
	})
	r.HandleFunc("/health", fc.handleHealth).Methods("GET")
	r.HandleFunc("/status", fc.handleGetStatus).Methods("GET")
	r.HandleFunc("/metrics", fc.handleGetMetrics).Methods("GET")
	r.HandleFunc("/tasks", fc.handleSubmitTask).Methods("POST")
	r.HandleFunc("/tasks/{id}", fc.handleGetTask).Methods("GET")

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("Shutting down server...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v\n", err)
		}
	}()

	log.Printf("Fog computing node %s listening on port %s\n", nodeID, port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v\n", err)
	}
}
