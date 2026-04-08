module TaskOneWorker

go 1.25.6

replace TaskOneUtils => ../utils

require TaskOneUtils v0.0.0-00010101000000-000000000000

require github.com/rabbitmq/amqp091-go v1.10.0 // indirect
