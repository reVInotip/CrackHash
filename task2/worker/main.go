package main

import (
	"log"
	"net/http"
	"strings"

	conf "TaskOneUtils/configuration"
	server "TaskOneUtils/http_server"
	def "TaskOneUtils/configuration/default_configs"
	worker "TaskOneWorker/worker"
)



func main() {
	// Setup config
	conf.ConfigurationSources = []conf.ConfigSource{
		{Name: "EnvConfig", CreateHandle: def.NewEnvConfig},
	}
	conf.InitGlobalConfig()

	// Set defaults
	if _, ok := conf.GetConfParam[string](conf.GlobalConfig, "listen_addr"); !ok {
		conf.AddConfParam(conf.GlobalConfig, "listen_addr", "0.0.0.0")
	}
	if _, ok := conf.GetConfParam[int](conf.GlobalConfig, "port"); !ok {
		conf.AddConfParam(conf.GlobalConfig, "port", 8081)
	}
	managerURL, ok := conf.GetConfParam[string](conf.GlobalConfig, "MANAGER_URL")
	if !ok {
		log.Fatal("MANAGER_URL environment variable required")
	}
	managerURL = strings.TrimSuffix(managerURL, "/")

	wkr := worker.NewWorker(managerURL)
	srv := server.NewServer("worker")
	srv.RegisterHandler(http.MethodPost, "/internal/api/hash/crack/task", wkr.HandleTask)

	log.Fatal(srv.ServerLoop())
}