package message_broker

type MessageBroker interface {
	AddQueue(name string) error
	SendMessage(queueName string, msgId string, contentType string, body []byte) error
	RecvMessageLoop(queueName string, recvCallback func([]byte)) error
	CountConsumers(queueName string) int
	GetAllMessages(queueName string) (chan []byte, error) 
	Close()
}