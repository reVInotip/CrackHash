package rabbitmq

import (
	"context"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQClient struct {
    conn    *amqp.Connection
    channel *amqp.Channel
    queues  map[string]amqp.Queue
    url     string
}

func NewClient(url string) (*RabbitMQClient, error) {
    client := &RabbitMQClient{
        url: url,
        queues: make(map[string]amqp.Queue),
    }
    
    if err := client.connect(); err != nil {
        return nil, err
    }
    
    return client, nil
}

func (r *RabbitMQClient) AddQueue(name string) {
    q, err := r.channel.QueueDeclare(
        name, // name
        true,         // durability
        false,        // delete when unused
        false,        // exclusive
        false,        // no-wait
        amqp.Table{
                amqp.QueueTypeArg: amqp.QueueTypeQuorum,
        },
    )
    if err != nil {
        log.Panicf("Failed to declare a queue: %s", err)
    }

    r.queues[name] = q
}

func (r *RabbitMQClient) SendMessage(queueName string, body []byte) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    err := r.channel.PublishWithContext(ctx,
        "",           // exchange
        queueName,       // routing key
        false,        // mandatory
        false,
        amqp.Publishing{
            DeliveryMode: amqp.Persistent,
            ContentType:  "application/json",
            Body:         body,
        })
    if err != nil {
        log.Panicf("Failed to send a message: %s", err)
    }

    log.Printf(" [x] Sent %s", body)
}

func (r *RabbitMQClient) RecvMessageLoop(queueName string, recvCallback func([]byte)) {
    err := r.channel.Qos(
        1,     // prefetch count
        0,     // prefetch size
        false, // global
    )

    if err != nil {
        log.Panicf("Failed to set QoS: %s", err)
    }

    msgs, err := r.channel.Consume(
        queueName, // queue
        "",     // consumer
        false,  // auto-ack
        false,  // exclusive
        false,  // no-local
        false,  // no-wait
        nil,    // args
    )

    if err != nil {
        log.Panicf("Failed to register a consumer: %s", err)
    }

    go func() {
        for d := range msgs {
            log.Printf("Received a message: %s", d.Body)
            recvCallback(d.Body)
            log.Printf("Done")
            d.Ack(false)
        }
    }()
}

func (r *RabbitMQClient) CountConsumers(queueName string) int {
    return r.queues[queueName].Consumers;
}

func (r *RabbitMQClient) connect() error {
    var err error
    
    r.conn, err = amqp.Dial(r.url)
    if err != nil {
        return err
    }
    
    r.channel, err = r.conn.Channel()
    if err != nil {
        return err
    }
    
    return nil
}

func (r *RabbitMQClient) Close() {
    if r.channel != nil {
        r.channel.Close()
    }
    if r.conn != nil {
        r.conn.Close()
    }
}