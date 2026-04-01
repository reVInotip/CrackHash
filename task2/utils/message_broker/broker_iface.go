package message_broker

type MessageBroker interface {
	AddQueue(name string)
	SendMessage(queueName string, body []byte)
	RecvMessageLoop(queueName string, recvCallback func([]byte))
	CountConsumers(queueName string) int
	Close()
}