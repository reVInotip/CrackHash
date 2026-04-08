package main

import (
	"log"
	"net/http"
	"sync"
	"time"

	manager "TaskOneManager/manager"
	config "TaskOneUtils/configuration"
	def "TaskOneUtils/configuration/default_configs"
	mongo "TaskOneUtils/db/mongodb"
	server "TaskOneUtils/http_server"
	rabbitMQ "TaskOneUtils/message_broker/rabbitmq"
)

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
	countWorkers, ok := config.GetConfParam[int](config.GlobalConfig, "COUNT_WORKERS")
	if !ok {
		log.Fatal("COUNT_WORKERS environment variable required")
	}
	timeoutSec, ok := config.GetConfParam[int](config.GlobalConfig, "REQUEST_TIMEOUT")
	if !ok {
		timeoutSec = 60 // default 60 seconds
	}
	timeout := time.Duration(timeoutSec) * time.Second

	alphabet, ok := config.GetConfParam[string](config.GlobalConfig, "ALPHABET")
	if !ok {
		alphabet = manager.DefaultAlphabet
	}
	cacheSize, ok := config.GetConfParam[int](config.GlobalConfig, "CACHE_SIZE")
	if !ok {
		cacheSize = manager.DefaultCacheSize
	}
	queueLen, ok := config.GetConfParam[int](config.GlobalConfig, "QUEUE_LEN")
	if !ok {
		queueLen = manager.DefaultQueueLen
	}
	maxConcurrentQueries, ok := config.GetConfParam[int](config.GlobalConfig, "MAX_CONCURRENT_QUERIES")
	if !ok {
		maxConcurrentQueries = manager.DefaultMaxConcurrentQueries
	}
	dbURI, ok := config.GetConfParam[string](config.GlobalConfig, "DATABASE_URI")
	if !ok {
		log.Fatal("Can not start without database URI")
	}
	brokerURI, ok := config.GetConfParam[string](config.GlobalConfig, "BROKER_URI")
	if !ok {
		log.Fatal("Can not start without message broker URI")
	}
	dbClient, err := mongo.NewClient(dbURI, "crack_requests")
	if err != nil {
		log.Fatalf("Can not establish connection to database: %s\n", err)
	}
	broker, err := rabbitMQ.NewClient(brokerURI)
	if err != nil {
		log.Fatalf("Can not establish connection to message broker: %s\n", err)
	}
	err = broker.AddQueue("manager_requests")
	if err != nil {
		log.Fatalf("Can not create queue in message broker: %s\n", err)
	}
	err = broker.AddQueue("worker_responses")
	if err != nil {
		log.Fatalf("Can not create queue in message broker: %s\n", err)
	}

	defer dbClient.Close()
	defer broker.Close()

	mgr := manager.NewManager(countWorkers, timeout,
		queueLen, alphabet, cacheSize,
		maxConcurrentQueries, dbClient, broker)

	srv := server.NewServer("manager")
	srv.RegisterHandler(http.MethodPost, "/api/hash/crack", mgr.HandleCrack)
	srv.RegisterHandler(http.MethodGet, "/api/hash/status", mgr.HandleStatus)

	var wg sync.WaitGroup
    
    wg.Go(func() {
        if err := srv.ServerLoop(); err != nil {
            log.Fatal(err)
        }
    })
    
    wg.Go(func() {
        if err := broker.RecvMessageLoop("worker_responses", mgr.HandleWorkerResponse); err != nil {
            log.Fatal(err)
        }
    })
    
    wg.Wait()
}