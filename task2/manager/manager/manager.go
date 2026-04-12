package manager

import (
	"TaskOneUtils/models"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	db "TaskOneUtils/db"
	ent "TaskOneUtils/db/entities"
	br "TaskOneUtils/message_broker"
)

const DefaultAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
const DefaultCacheSize = 50
const DefaultQueueLen = 1000
const DefaultMaxConcurrentQueries = 50

type requestStatus string

const (
	statusInProgress requestStatus = "IN_PROGRESS"
	statusPending	 requestStatus = "PENDING"
	statusReady      requestStatus = "READY"
	statusError      requestStatus = "ERROR"
)

type requestInfo struct {
	id				 string
	status           requestStatus
	hash			 string
	maxLen			 int
	words            []string
	partCount        int
	receivedParts    map[int]bool // track which parts have responded
	mu               sync.RWMutex
	timeoutTimer     *time.Timer
}

type manager struct {
	requests 	 		   map[string]*requestInfo
	requestCache 		   map[string]string
	requestQueue 		   chan *requestInfo
	countConcurrentQueries int32
	mu           		   sync.RWMutex
	timeout      		   time.Duration
	alphabet			   string
	cacheSize			   int
	queueLen		 	   int
	maxConcurrentQueries   int32
	databaseClient 		   db.Database
	msgBroker			   br.MessageBroker
	countWorkers		   int
}

func NewManager(countWorkers int, timeout time.Duration, queueLen int, alphabet string, cacheSize int,
	maxConcurrentQueries int, databaseClient db.Database, msgBroker br.MessageBroker) *manager {
	m := &manager{
		requests:     		    make(map[string]*requestInfo),
		requestCache: 		    make(map[string]string),
		requestQueue: 		    make(chan *requestInfo, queueLen),
		countConcurrentQueries: 0,
		timeout:      		    timeout,
		alphabet:	  		    alphabet,
		cacheSize:	  		    cacheSize,
		queueLen:	  		    queueLen,
		maxConcurrentQueries:   int32(maxConcurrentQueries),
		databaseClient: 		databaseClient,
		msgBroker: 				msgBroker,
		countWorkers: 			countWorkers,
	}

	m.recoveryManager()

	return m
}

func (m *manager) HandleCrack(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handle crack request")
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

	currentWorkersCount := m.msgBroker.CountConsumers("manager_requests")
	if currentWorkersCount == 0 {
		http.Error(w, "Server don`t have enough workers, try again later", http.StatusServiceUnavailable)
		return
	}

	partCount := m.countWorkers
	if partCount == 0 {
		http.Error(w, "No workers configured", http.StatusInternalServerError)
		return
	}

	// Store request
	info := &requestInfo{
		id:			   uuid.New().String(),
		status:        statusInProgress,
		hash: 		   req.Hash,
		maxLen:		   req.MaxLength,
		words:         nil,
		partCount:     partCount,
		receivedParts: make(map[int]bool),
	}

	m.mu.Lock()
	m.requests[info.id] = info

	if m.countConcurrentQueries == m.maxConcurrentQueries {
		log.Println("Add request to queue");

		// Add request to queue (channels already thread-safety)
		select {
		case m.requestQueue <- info:
			// we don't need to lock info here because it only
			// owners are we and requestQueue that we already lock exclusive
			info.status = statusPending

			resp := models.CrackResponse{RequestID: info.id}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(resp)

			m.mu.Unlock()
			return
		default:
			http.Error(w, "Server is busy, try again later", http.StatusServiceUnavailable)

			m.mu.Unlock()
			return
		}
	}
	m.mu.Unlock()

	// Store request info to database
	dbInfo := requestInfoToRequest(info)
	err := m.databaseClient.CreateRequestWithoutConnRetry(dbInfo)
	if err != nil {
		http.Error(w, "Can not store request to persistent storage", http.StatusInternalServerError)
		return
	}

	m.processTask(info)

	resp := models.CrackResponse{RequestID: info.id}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func (m *manager) processTask(info *requestInfo) {
	m.mu.Lock()
	m.countConcurrentQueries++
	m.mu.Unlock()

	info.mu.Lock()
	defer info.mu.Unlock()

	info.status = statusInProgress

	// Set timeout
	info.timeoutTimer = time.AfterFunc(m.timeout, func() {
		m.mu.Lock()
		if ri, ok := m.requests[info.id]; ok && ri.status == statusInProgress {
			ri.status = statusError
			ri.words = nil
			log.Printf("Request %s timed out", info.id)
		}
		m.mu.Unlock()

		m.updateQueue()
	})

	partCount := m.countWorkers

	// Send tasks to workers (1-based)
	for i := 1; i <= partCount; i++ {
		if info.receivedParts[i] {
			continue
		}

		task := models.WorkerTask{
			RequestID:  info.id,
			PartNumber: i,
			PartCount:  info.partCount,
			Hash:       info.hash,
			MaxLength:  info.maxLen,
			Alphabet:   m.alphabet,
		}
		go m.sendTask(task) // async
	}
}

func (m *manager) sendTask(task models.WorkerTask) {
	data, err := json.Marshal(task)
	if err != nil {
		log.Printf("Failed to marshal task: %v", err)
		return
	}

	err = m.msgBroker.SendMessage("manager_requests", "application/json", data)
	if err != nil {
		log.Printf("Can not send message to broker: %s", err)
	}
}

func (m *manager) HandleStatus(w http.ResponseWriter, r *http.Request) {
	log.Printf("Handle status request")
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

func (m *manager) HandleWorkerResponse(body []byte) {
	// Expect XML body
	var workerResp models.WorkerResponse
	if err := xml.NewDecoder(bytes.NewReader(body)).Decode(&workerResp); err != nil {
		log.Println("Invalid XML from worker response")
		return
	}
	requestID := workerResp.RequestID
	partNumber := workerResp.PartNumber
	words := workerResp.Words

	m.mu.RLock()
	info, ok := m.requests[requestID]
	m.mu.RUnlock()
	if !ok {
		log.Printf("Received response for unknown request %s\n", requestID)
		return
	}

	if info.status != statusInProgress {
		log.Printf("Received response for completed or error request %s\n", requestID)
		return
	}

	// Update request info in database
	m.databaseClient.UpdateRequestReceivedPartsAndWords(requestID, partNumber, words)

	info.mu.Lock()
	defer info.mu.Unlock()
	if info.status != statusInProgress {
		// already done or error
		log.Printf("Received response with unexpected status %s\n", requestID)
		return
	}
	// Mark part as received
	if info.receivedParts[partNumber] {
		// duplicate, ignore
		log.Printf("Received duplicate part for request %s\n", requestID)
		return
	}
	info.receivedParts[partNumber] = true
	info.words = append(info.words, words...)

	// Check if all parts received
	if len(info.receivedParts) == info.partCount {
		// Remove request info from database
		m.databaseClient.DeleteRequest(requestID)

		info.status = statusReady

		m.updateCache(requestID, info)
		m.updateQueue()
		
		if info.timeoutTimer != nil {
			info.timeoutTimer.Stop()
		}
		log.Printf("Request %s completed", requestID)
	}
}

func (m *manager) updateCache(requestID string, info *requestInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.requestCache) + 1 > m.cacheSize {
		var key string
		for k, _ := range m.requestCache {
			key = k
			break
		}

		delete(m.requestCache, key)
	}
	m.requestCache[info.hash] = requestID
}

func (m *manager) updateQueue() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.requestQueue) > 0 {
		req := <-m.requestQueue
		go m.processTask(req)
	} else if m.countConcurrentQueries > 0 {
		m.countConcurrentQueries--;
	} else {
		log.Fatal("Count concurrent queries can not be less then zero")
	}
}

func (m *manager) recoveryManager() {
	requests, err := m.databaseClient.GetAllRequests()
	if err != nil {
		log.Printf("Failed recovery manager state: %s\n", err)
		return
	}

	if len(requests) == 0 {
		log.Println("Not found any running requests")
		return
	}

	for i, request := range requests {
		info := &requestInfo{
			id:			   request.RequestID,
			status:        statusInProgress,
			hash: 		   request.Hash,
			maxLen:		   request.MaxLen,
			words:         request.Words,
			partCount:     request.MaxLen,
			receivedParts: request.ReceivedParts,
		}

		m.mu.Lock()
		m.requests[info.id] = info
		m.mu.Unlock()

		if i < int(m.maxConcurrentQueries) {
			go m.processTask(info)
		} else {
			info.status = statusPending
			m.requestQueue <- info
		}
	}

	log.Println("Recovery finish successfully")
}

func requestInfoToRequest(info *requestInfo) *ent.Request {
	return &ent.Request{
		RequestID: info.id,
		Hash: info.hash,
		MaxLen: info.maxLen,
		Words: info.words,
		PartCount: info.partCount,
		ReceivedParts: info.receivedParts,
	}
}