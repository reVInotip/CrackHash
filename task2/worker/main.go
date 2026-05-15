package main

import (
	"log"
	"strings"

	conf "TaskOneUtils/configuration"
	def "TaskOneUtils/configuration/default_configs"
	worker "TaskOneWorker/worker"
	rabbitMQ "TaskOneUtils/message_broker/rabbitmq"
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
	brokerURI, ok := conf.GetConfParam[string](conf.GlobalConfig, "BROKER_URI")
	if !ok {
		log.Fatal("Can not start without message broker URI")
	}
	brokerURI = strings.TrimSuffix(brokerURI, "/")

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

	defer broker.Close()

	wkr := worker.NewWorker(broker)

	go log.Fatal(broker.RecvMessageLoop("manager_requests", wkr.HandleTask))
}