package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	config "TaskOneUtils/configuration"
	def "TaskOneUtils/configuration/default_configs"
	server "TaskOneUtils/http_server"
	manager "TaskOneManager/manager"
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

	mgr := manager.NewManager(workerURLs, timeout)

	srv := server.NewServer("manager")
	srv.RegisterHandler(http.MethodPost, "/api/hash/crack", mgr.HandleCrack)
	srv.RegisterHandler(http.MethodGet, "/api/hash/status", mgr.HandleStatus)
	srv.RegisterHandler(http.MethodPost, "/internal/api/hash/crack/request", mgr.HandleWorkerResponse)

	log.Fatal(srv.ServerLoop())
}