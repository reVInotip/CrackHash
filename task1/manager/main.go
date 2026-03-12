package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	config "TaskOneUtils/configuration"
	def "TaskOneUtils/configuration/default_configs"
	server "TaskOneUtils/http_server"
	"TaskOneUtils/models"

	"github.com/google/uuid"
)

const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
const cacheSize = 50
const queueLen = 50

type requestStatus string

const (
	statusInProgress requestStatus = "IN_PROGRESS"
	statusReady      requestStatus = "READY"
	statusError      requestStatus = "ERROR"
)

type requestInfo struct {
	status           requestStatus
	hash			 string
	words            []string
	partCount        int
	receivedParts    map[int]bool // track which parts have responded
	mu               sync.RWMutex
	timeoutTimer     *time.Timer
}

type manager struct {
	requests 	 map[string]*requestInfo
	requestCache map[string]string
	requestQueue chan models.CrackRequest
	mu           sync.RWMutex
	client       *http.Client
	workerURLs   []string
	timeout      time.Duration
}

func newManager(workerURLs []string, timeout time.Duration) *manager {
	return &manager{
		requests:     make(map[string]*requestInfo),
		requestCache: make(map[string]string),
		requestQueue: make(chan models.CrackRequest),
		client:       &http.Client{Timeout: 10 * time.Second},
		workerURLs:   workerURLs,
		timeout:      timeout,
	}
}

func (m *manager) handleCrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req models.CrackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// Validate
	if req.Hash == "" || req.MaxLength <= 0 {
		http.Error(w, "hash and maxLength required", http.StatusBadRequest)
		return
	}

	// Add request to queue (channels already thread-safety)
	m.requestQueue <- req

	m.mu.RLock()
	cachedReqID, ok := m.requestCache[req.Hash]
	m.mu.RUnlock()
	if ok {
		log.Printf("Return cached results for hash %s\n", req.Hash);

		resp := models.CrackResponse{RequestID: cachedReqID};
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Generate request ID
	requestID := uuid.New().String()

	partCount := len(m.workerURLs)
	if partCount == 0 {
		http.Error(w, "No workers configured", http.StatusInternalServerError)
		return
	}

	// Store request
	info := &requestInfo{
		status:        statusInProgress,
		hash: 		   req.Hash,
		words:         nil,
		partCount:     partCount,
		receivedParts: make(map[int]bool),
	}
	m.mu.Lock()
	m.requests[requestID] = info
	m.mu.Unlock()

	// Set timeout
	info.timeoutTimer = time.AfterFunc(m.timeout, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if ri, ok := m.requests[requestID]; ok && ri.status == statusInProgress {
			ri.status = statusError
			ri.words = nil
			log.Printf("Request %s timed out", requestID)
		}
	})

	// Send tasks to workers
	for i, workerURL := range m.workerURLs {
		partNumber := i + 1 // 1-based
		task := models.WorkerTask{
			RequestID:  requestID,
			PartNumber: partNumber,
			PartCount:  partCount,
			Hash:       req.Hash,
			MaxLength:  req.MaxLength,
			Alphabet:   alphabet,
		}
		go m.sendTask(workerURL, task) // async
	}

	resp := models.CrackResponse{RequestID: requestID}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (m *manager) sendTask(workerURL string, task models.WorkerTask) {
	url := workerURL + "/worker/internal/api/hash/crack/task"
	data, err := json.Marshal(task)
	if err != nil {
		log.Printf("Failed to marshal task: %v", err)
		return
	}
	resp, err := m.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("Failed to send task to worker %s: %v", workerURL, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Worker %s returned error: %s", workerURL, body)
	}
}

func (m *manager) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	requestID := r.URL.Query().Get("requestId")
	if requestID == "" {
		http.Error(w, "Missing requestId", http.StatusBadRequest)
		return
	}
	m.mu.RLock()
	info, ok := m.requests[requestID]
	m.mu.RUnlock()
	if !ok {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}
	info.mu.RLock()
	status := info.status
	var data []string
	if status == statusReady {
		// copy words
		data = make([]string, len(info.words))
		copy(data, info.words)
	}
	info.mu.RUnlock()

	resp := models.StatusResponse{
		Status: string(status),
		Data:   data,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (m *manager) handleWorkerResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Expect XML body
	var workerResp models.WorkerResponse
	if err := xml.NewDecoder(r.Body).Decode(&workerResp); err != nil {
		http.Error(w, "Invalid XML", http.StatusBadRequest)
		return
	}
	requestID := workerResp.RequestID
	partNumber := workerResp.PartNumber
	words := workerResp.Words

	m.mu.RLock()
	info, ok := m.requests[requestID]
	m.mu.RUnlock()
	if !ok {
		log.Printf("Received response for unknown request %s", requestID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	info.mu.Lock()
	defer info.mu.Unlock()
	if info.status != statusInProgress {
		// already done or error
		w.WriteHeader(http.StatusOK)
		return
	}
	// Mark part as received
	if info.receivedParts[partNumber] {
		// duplicate, ignore
		w.WriteHeader(http.StatusOK)
		return
	}
	info.receivedParts[partNumber] = true
	info.words = append(info.words, words...)

	// Check if all parts received
	if len(info.receivedParts) == info.partCount {
		info.status = statusReady

		// update cache
		if len(m.requestCache) + 1 > cacheSize {
			var key string
			for k, _ := range m.requestCache {
				key = k
				break
			}

			delete(m.requestCache, key)
		}
		m.requestCache[info.hash] = requestID
		
		if info.timeoutTimer != nil {
			info.timeoutTimer.Stop()
		}
		log.Printf("Request %s completed", requestID)
	}
	w.WriteHeader(http.StatusOK)
}

func main() {
	// Setup config sources
	config.ConfigurationSources = []config.ConfigSource{
		{Name: "EnvConfig", CreateHandle: def.NewEnvConfig},
	}
	config.InitGlobalConfig()

	// Set defaults if not provided by env
	if _, ok := config.GetConfParam[string](config.GlobalConfig, "listen_addr"); !ok {
		config.AddConfParam(config.GlobalConfig, "listen_addr", "0.0.0.0")
	}
	if _, ok := config.GetConfParam[int](config.GlobalConfig, "port"); !ok {
		config.AddConfParam(config.GlobalConfig, "port", 8080)
	}
	workerURLsStr, ok := config.GetConfParam[string](config.GlobalConfig, "WORKER_URLS")
	if !ok {
		log.Fatal("WORKER_URLS environment variable required (comma-separated)")
	}
	workerURLs := strings.Split(workerURLsStr, ",")
	for i, u := range workerURLs {
		workerURLs[i] = strings.TrimSpace(u)
	}
	timeoutSec, ok := config.GetConfParam[int](config.GlobalConfig, "REQUEST_TIMEOUT")
	if !ok {
		timeoutSec = 60 // default 60 seconds
	}
	timeout := time.Duration(timeoutSec) * time.Second

	mgr := newManager(workerURLs, timeout)

	srv := server.NewServer("manager")
	srv.RegisterHandler(http.MethodPost, "/api/hash/crack", mgr.handleCrack)
	srv.RegisterHandler(http.MethodGet, "/api/hash/status", mgr.handleStatus)
	srv.RegisterHandler(http.MethodPost, "/internal/api/hash/crack/request", mgr.handleWorkerResponse)

	log.Fatal(srv.ServerLoop())
}