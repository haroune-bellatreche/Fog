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
	MaxLoadThreshold = 0.8 // Rejeter les tâches si la charge > 80%
)

// FogNode représente un nœud de fog computing
type FogNode struct {
	ID       string    `json:"id"`
	Location string    `json:"location"`
	Status   string    `json:"status"`
	Load     float64   `json:"load"`
	LastSeen time.Time `json:"last_seen"`
}

// Task représente une tâche computationnelle
type Task struct {
	ID          string                 `json:"id"`
	Type        string                 `json:"type"`
	Payload     map[string]interface{} `json:"payload"`
	Priority    int                    `json:"priority"`                // Priorité originale du client
	Criticality int                    `json:"criticality"`             // 1-5, plus élevé = plus critique
	SmartScore  float64                `json:"smart_score"`             // Score intelligent calculé
	EstimatedLatency time.Duration     `json:"estimated_latency,omitempty"`
	CPUCost     float64                `json:"cpu_cost,omitempty"`      // Utilisation CPU estimée (0.0-1.0)
	RAMCost     float64                `json:"ram_cost,omitempty"`      // Utilisation RAM estimée (0.0-1.0)
	StorageCost float64                `json:"storage_cost,omitempty"`  // Utilisation stockage estimée (MB)
	EnergyCost  float64                `json:"energy_cost,omitempty"`   // Consommation énergie estimée (Wh)
	NetworkLatency time.Duration       `json:"network_latency,omitempty"` // Latence réseau vers le nœud
	Status      string                 `json:"status"`
	Result      interface{}            `json:"result,omitempty"`
	SubmittedAt time.Time              `json:"submitted_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// RejectedTask représente une tâche rejetée avec sa raison
type RejectedTask struct {
	Task         Task      `json:"task"`
	RejectedAt   time.Time `json:"rejected_at"`
	RejectionReason string `json:"rejection_reason"`
	NodeLoad     float64   `json:"node_load"`
	QueueSize    int       `json:"queue_size"`
}

// TaskHeap implémente un min-heap basé sur le score intelligent
// Score plus bas = priorité plus haute pour l'exécution
type TaskHeap []*Task

func (h TaskHeap) Len() int           { return len(h) }
func (h TaskHeap) Less(i, j int) bool { 
	// Utilise SmartScore pour la comparaison
	return h[i].SmartScore < h[j].SmartScore
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

// calculateScore calcule le score intelligent de planification
// Score plus bas = doit être exécuté en premier
// Considère: priorité, criticité, latence, utilisation des ressources, efficacité énergétique
func (t *Task) calculateScore() float64 {
	baseScore := float64(t.Priority)
	criticalityBonus := float64(5 - t.Criticality) * 10 // Criticité plus haute réduit le score
	latencyPenalty := t.EstimatedLatency.Seconds() * 0.1
	networkPenalty := t.NetworkLatency.Seconds() * 0.05

	// Efficacité des ressources: préfère les tâches qui utilisent moins de ressources
	resourcePenalty := (t.CPUCost + t.RAMCost) * 5
	storagePenalty := t.StorageCost * 0.001

	// Efficacité énergétique: préfère la faible consommation d'énergie
	energyPenalty := t.EnergyCost * 2

	return baseScore + criticalityBonus + latencyPenalty + networkPenalty +
		   resourcePenalty + storagePenalty + energyPenalty
}

// FogCompute gère les opérations de fog computing
type FogCompute struct {
	node    FogNode
	tasks   map[string]*Task
	taskHeap TaskHeap
	rejectedTasks []RejectedTask  // Queue pour les tâches rejetées
	mu      sync.RWMutex
	cond    *sync.Cond
	metrics Metrics
	// Ressources disponibles
	availableCPU    float64
	availableRAM    float64
	availableStorage float64
	energyLevel     float64 // Niveau d'énergie actuel (0.0-1.0)
}

// Metrics suit les métriques de performance
type Metrics struct {
	TasksProcessed int           `json:"tasks_processed"`
	TasksRejected  int           `json:"tasks_rejected"`  // Compteur de tâches rejetées
	AvgLatency     time.Duration `json:"avg_latency"`
	CurrentLoad    float64       `json:"current_load"`
	mu             sync.RWMutex
}

// NewFogCompute crée une nouvelle instance de fog computing
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
		rejectedTasks: make([]RejectedTask, 0),  // Initialiser la queue des tâches rejetées
		metrics: Metrics{
			TasksProcessed: 0,
			TasksRejected:  0,
			AvgLatency:     0,
			CurrentLoad:    0.0,
		},
		// Initialiser les ressources disponibles
		availableCPU:     1.0,  // 100% CPU disponible
		availableRAM:     1.0,  // 100% RAM disponible
		availableStorage: 1000.0, // 1000 MB stockage disponible
		energyLevel:      1.0,  // 100% niveau d'énergie
	}
	fc.cond = sync.NewCond(&fc.mu)
	heap.Init(&fc.taskHeap)
	return fc
}

// rejectTask sauvegarde une tâche rejetée dans la queue des rejets
func (fc *FogCompute) rejectTask(task Task, reason string, load float64, queueSize int) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	rejectedTask := RejectedTask{
		Task:            task,
		RejectedAt:      time.Now(),
		RejectionReason: reason,
		NodeLoad:        load,
		QueueSize:       queueSize,
	}

	fc.rejectedTasks = append(fc.rejectedTasks, rejectedTask)
	
	fc.metrics.mu.Lock()
	fc.metrics.TasksRejected++
	fc.metrics.mu.Unlock()

	log.Printf("Tâche rejetée et sauvegardée: ID=%s, Priority=%d, SmartScore=%.2f, Raison=%s, Charge=%.2f, TailleQueue=%d\n", 
		task.ID, task.Priority, task.SmartScore, reason, load, queueSize)
}

// Start commence le traitement des tâches
func (fc *FogCompute) Start(ctx context.Context) {
	log.Println("Démarrage du nœud fog computing:", fc.node.ID)
	
	// Démarrer le pool de workers
	numWorkers := 5
	for i := 0; i < numWorkers; i++ {
		go fc.worker(ctx, i)
	}

	// Démarrer le mise à jour des métriques
	go fc.updateMetrics(ctx)
}

// worker traite les tâches depuis la priority queue
func (fc *FogCompute) worker(ctx context.Context, workerID int) {
	log.Printf("Worker %d démarré\n", workerID)
	
	for {
		fc.mu.Lock()
		for fc.taskHeap.Len() == 0 {
			fc.cond.Wait() // Attendre que des tâches soient disponibles
		}
		task := heap.Pop(&fc.taskHeap).(*Task)
		fc.mu.Unlock()
		
		select {
		case <-ctx.Done():
			log.Printf("Worker %d en arrêt\n", workerID)
			return
		default:
			fc.processTask(task)
		}
	}
}

// processTask exécute une tâche unique
func (fc *FogCompute) processTask(task *Task) {
	startTime := time.Now()
	
	fc.mu.Lock()
	task.Status = "processing"
	fc.mu.Unlock()

	log.Printf("Traitement tâche %s type %s (priority=%d, criticality=%d, smart_score=%.2f)\n", 
		task.ID, task.Type, task.Priority, task.Criticality, task.SmartScore)

	// Simuler différents types de tâches de fog computing
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
		result = map[string]string{"error": "type de tâche inconnu"}
	}

	completedAt := time.Now()
	latency := completedAt.Sub(startTime)

	fc.mu.Lock()
	task.Status = "completed"
	task.Result = result
	task.CompletedAt = &completedAt

	// Libérer les ressources
	fc.availableCPU += task.CPUCost
	fc.availableRAM += task.RAMCost
	fc.availableStorage += task.StorageCost
	fc.energyLevel += task.EnergyCost

	fc.mu.Unlock()

	// Mettre à jour les métriques
	fc.metrics.mu.Lock()
	fc.metrics.TasksProcessed++
	if fc.metrics.AvgLatency == 0 {
		fc.metrics.AvgLatency = latency
	} else {
		fc.metrics.AvgLatency = (fc.metrics.AvgLatency + latency) / 2
	}
	fc.metrics.mu.Unlock()

	log.Printf("Tâche %s complétée en %v (priority=%d, smart_score=%.2f)\n", 
		task.ID, latency, task.Priority, task.SmartScore)
}

// Opérations simulées de fog computing
func (fc *FogCompute) aggregateData(payload map[string]interface{}) map[string]interface{} {
	time.Sleep(100 * time.Millisecond) // Simuler le traitement
	return map[string]interface{}{
		"operation": "data_aggregation",
		"status":    "success",
		"summary":   "Données agrégées de plusieurs sources de capteurs",
		"count":     42,
	}
}

func (fc *FogCompute) performAnalytics(payload map[string]interface{}) map[string]interface{} {
	time.Sleep(200 * time.Millisecond) // Simuler le traitement
	return map[string]interface{}{
		"operation": "edge_analytics",
		"status":    "success",
		"insights":  "Anomalie détectée dans les lectures de capteurs",
		"confidence": 0.87,
	}
}

func (fc *FogCompute) preprocessData(payload map[string]interface{}) map[string]interface{} {
	time.Sleep(50 * time.Millisecond) // Simuler le traitement
	return map[string]interface{}{
		"operation": "preprocessing",
		"status":    "success",
		"filtered":  true,
		"normalized": true,
	}
}

func (fc *FogCompute) cacheData(payload map[string]interface{}) map[string]interface{} {
	time.Sleep(30 * time.Millisecond) // Simuler le traitement
	return map[string]interface{}{
		"operation": "caching",
		"status":    "success",
		"cached":    true,
		"ttl":       3600,
	}
}

// updateMetrics met à jour périodiquement les métriques du nœud
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

// Gestionnaires HTTP
func (fc *FogCompute) handleSubmitTask(w http.ResponseWriter, r *http.Request) {
	var task Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Planification intelligente: vérifier la charge actuelle et les ressources disponibles
	fc.mu.RLock()
	currentLoad := fc.node.Load
	queueSize := fc.taskHeap.Len()
	availableCPU := fc.availableCPU
	availableRAM := fc.availableRAM
	availableStorage := fc.availableStorage
	energyLevel := fc.energyLevel
	fc.mu.RUnlock()


	// Définir les valeurs par défaut pour les coûts de ressources
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

	// NOUVEAU: Calculer et assigner le SmartScore AVANT toute vérification
	task.SmartScore = task.calculateScore()

	task.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	task.SubmittedAt = time.Now()

	// Vérifier les conditions de rejet et sauvegarder les tâches rejetées
	if currentLoad > MaxLoadThreshold || queueSize > 50 {
		task.Status = "rejected"
		reason := fmt.Sprintf("Nœud surchargé: charge=%.2f, taille_queue=%d", currentLoad, queueSize)
		fc.rejectTask(task, reason, currentLoad, queueSize)
		
		http.Error(w, reason, http.StatusServiceUnavailable)
		return
	}

	// Vérifier la disponibilité des ressources
	if task.CPUCost > availableCPU || task.RAMCost > availableRAM || task.StorageCost > availableStorage {
		task.Status = "rejected"
		reason := fmt.Sprintf("Ressources insuffisantes: CPU=%.2f/%.2f, RAM=%.2f/%.2f, Storage=%.2f/%.2f",
			task.CPUCost, availableCPU, task.RAMCost, availableRAM, task.StorageCost, availableStorage)
		fc.rejectTask(task, reason, currentLoad, queueSize)
		
		http.Error(w, reason, http.StatusServiceUnavailable)
		return
	}

	// Vérifier le niveau d'énergie pour les tâches critiques
	if task.Criticality >= 4 && energyLevel < 0.3 {
		task.Status = "rejected"
		reason := fmt.Sprintf("Niveau d'énergie bas pour tâche critique: énergie=%.2f", energyLevel)
		fc.rejectTask(task, reason, currentLoad, queueSize)
		
		http.Error(w, reason, http.StatusServiceUnavailable)
		return
	}

	task.Status = "queued"

	fc.mu.Lock()
	// Réserver les ressources
	fc.availableCPU -= task.CPUCost
	fc.availableRAM -= task.RAMCost
	fc.availableStorage -= task.StorageCost
	fc.energyLevel -= task.EnergyCost

	fc.tasks[task.ID] = &task
	heap.Push(&fc.taskHeap, &task)
	fc.cond.Signal() // Réveiller un worker en attente
	fc.mu.Unlock()

	log.Printf("Tâche %s soumise: type=%s, priority=%d, criticality=%d, smart_score=%.2f, latence_estimée=%v, ressources_réservées: CPU=%.2f, RAM=%.2f, Storage=%.2f, Energy=%.2f\n", 
		task.ID, task.Type, task.Priority, task.Criticality, task.SmartScore, task.EstimatedLatency, 
		task.CPUCost, task.RAMCost, task.StorageCost, task.EnergyCost)

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
		http.Error(w, "Tâche non trouvée", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// handleGetRejectedTasks retourne toutes les tâches rejetées
func (fc *FogCompute) handleGetRejectedTasks(w http.ResponseWriter, r *http.Request) {
	fc.mu.RLock()
	rejectedTasks := make([]RejectedTask, len(fc.rejectedTasks))
	copy(rejectedTasks, fc.rejectedTasks)
	fc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total": len(rejectedTasks),
		"tasks": rejectedTasks,
	})
}

// handleRetryRejectedTask permet de réessayer une tâche rejetée spécifique
func (fc *FogCompute) handleRetryRejectedTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	fc.mu.Lock()
	defer fc.mu.Unlock()

	// Trouver la tâche rejetée
	var foundIndex = -1
	var taskToRetry Task
	
	for i, rt := range fc.rejectedTasks {
		if rt.Task.ID == taskID {
			foundIndex = i
			taskToRetry = rt.Task
			break
		}
	}

	if foundIndex == -1 {
		http.Error(w, "Tâche rejetée non trouvée", http.StatusNotFound)
		return
	}

	// Vérifier si les ressources sont maintenant disponibles
	if taskToRetry.CPUCost > fc.availableCPU || taskToRetry.RAMCost > fc.availableRAM || 
	   taskToRetry.StorageCost > fc.availableStorage {
		http.Error(w, "Ressources toujours insuffisantes pour réessayer la tâche", http.StatusServiceUnavailable)
		return
	}

	// Retirer de la queue des rejets
	fc.rejectedTasks = append(fc.rejectedTasks[:foundIndex], fc.rejectedTasks[foundIndex+1:]...)

	// Mettre à jour le statut de la tâche et resoumettre
	taskToRetry.Status = "queued"
	taskToRetry.SubmittedAt = time.Now()
	// Recalculer le SmartScore au cas où les conditions auraient changé
	taskToRetry.SmartScore = taskToRetry.calculateScore()

	// Réserver les ressources
	fc.availableCPU -= taskToRetry.CPUCost
	fc.availableRAM -= taskToRetry.RAMCost
	fc.availableStorage -= taskToRetry.StorageCost
	fc.energyLevel -= taskToRetry.EnergyCost

	fc.tasks[taskToRetry.ID] = &taskToRetry
	heap.Push(&fc.taskHeap, &taskToRetry)
	fc.cond.Signal()

	log.Printf("Réessai de la tâche rejetée %s (priority=%d, smart_score=%.2f)\n", 
		taskID, taskToRetry.Priority, taskToRetry.SmartScore)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Tâche resoumise avec succès",
		"task":    taskToRetry,
	})
}

// handleClearRejectedTasks efface toutes les tâches rejetées
func (fc *FogCompute) handleClearRejectedTasks(w http.ResponseWriter, r *http.Request) {
	fc.mu.Lock()
	count := len(fc.rejectedTasks)
	fc.rejectedTasks = make([]RejectedTask, 0)
	fc.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Tâches rejetées effacées",
		"count":   count,
	})
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

	fc.mu.RLock()
	rejectedCount := len(fc.rejectedTasks)
	fc.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks_processed":      metrics.TasksProcessed,
		"tasks_rejected":       metrics.TasksRejected,
		"rejected_queue_size":  rejectedCount,
		"avg_latency_ms":       metrics.AvgLatency.Milliseconds(),
		"current_load":         metrics.CurrentLoad,
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

	// Configuration des routes HTTP
	r := mux.NewRouter()

	// Middleware CORS
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			
			// Gérer les requêtes preflight
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
	
	// Endpoints pour gérer les tâches rejetées
	r.HandleFunc("/rejected-tasks", fc.handleGetRejectedTasks).Methods("GET")
	r.HandleFunc("/rejected-tasks/{id}/retry", fc.handleRetryRejectedTask).Methods("POST")
	r.HandleFunc("/rejected-tasks", fc.handleClearRejectedTasks).Methods("DELETE")

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// Arrêt gracieux
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("Arrêt du serveur...")
		cancel()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Erreur d'arrêt du serveur: %v\n", err)
		}
	}()

	log.Printf("Nœud fog computing %s en écoute sur le port %s\n", nodeID, port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Erreur serveur: %v\n", err)
	}
}
