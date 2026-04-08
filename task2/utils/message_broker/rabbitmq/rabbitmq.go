package rabbitmq

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	msgBroker "TaskOneUtils/message_broker"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQClient struct {
    conn    *amqp.Connection
    channel *amqp.Channel
    queues  map[string]amqp.Queue
    url     string
    stop    chan os.Signal
}

func signalsSetup() chan os.Signal {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	return stop
}

func (r *RabbitMQClient) waitReconnect() chan os.Signal {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    arrived := make(chan os.Signal, 1)
    for {
        err := r.connect();

        if err == nil && !r.conn.IsClosed() {
            log.Println("Reconnect to RabbitMQ successfully")
            return arrived
        }

        log.Println("Reconnection to RabbitMQ failed, retry in 5 seconds...")

        select {
        case <- ticker.C:
            continue
        case s := <- r.stop:
            log.Println("Stopping signal requested")
            arrived <- s
            return arrived
        }
    }
}

func (r *RabbitMQClient) tryExecute(operation func() error) error {
    for {
        if r.conn != nil {
            err := operation()
            if err == nil {
                return nil
            }
            
            if !r.conn.IsClosed() {
                return err
            }
            
            log.Printf("Connection error during operation: %v, will retry", err)

            select {
            case <- r.waitReconnect():
                return nil
            case <- r.stop:
                log.Println("Stopping signal requested")
                return nil
            }
        }
    }
}

func NewClient(url string) (msgBroker.MessageBroker, error) {
    client := &RabbitMQClient{
        url: url,
        queues: make(map[string]amqp.Queue),
    }

    client.stop = signalsSetup()
    
    if err := client.connect(); err != nil {
        return nil, err
    }
    
    return client, nil
}

func (r *RabbitMQClient) AddQueue(name string) error {
    err := r.tryExecute(func() error {
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
            return err
        }

        r.queues[name] = q

        return nil
    })

    if err != nil {
        log.Fatalf("Can not create queue %s", err)
        return err
    }

    return nil
}

func (r *RabbitMQClient) SendMessage(queueName string, contentType string, body []byte) error {
    err := r.tryExecute(func() error {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        err := r.channel.PublishWithContext(ctx,
            "",           // exchange
            queueName,       // routing key
            false,        // mandatory
            false,
            amqp.Publishing{
                DeliveryMode: amqp.Persistent,
                ContentType:  contentType,
                Body:         body,
            })
        if err != nil {
            return err
        }

        return nil
    })

    if err != nil {
        log.Fatalf("Can not create queue %s", err)
        return err
    }

    log.Printf(" [x] Sent %s", body)
    return nil
}

func (r *RabbitMQClient) RecvMessageLoop(queueName string, recvCallback func([]byte)) error {
    err := r.channel.Qos(
        1,     // prefetch count
        0,     // prefetch size
        false, // global
    )

    if err != nil {
        log.Panicf("Failed to set QoS: %s", err)
    }

    err = r.tryExecute(func() error {
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
            return err
        }

        notifyClose := make(chan *amqp.Error, 1)
        r.conn.NotifyClose(notifyClose)

        requestComplete := make(chan *amqp.Delivery, 1)

        log.Println("RabbitMQ server started")
        for {
            select {
            case d, ok := <-msgs:
                if (!ok) {
                    return nil
                }
                log.Printf("Received a message: %s", d.Body)
                
                go func () {
                    recvCallback(d.Body)
                    requestComplete <- &d
                }()
            case err := <-notifyClose:
                return err
            case <- r.stop:
                log.Println("Stopping signal requested")
                return nil
            case d := <- requestComplete:
                log.Printf("Done")
                err := d.Ack(false)
                if err != nil {
                    return err
                }
            }
        }
    })

    return nil
}

func (r *RabbitMQClient) CountConsumers(queueName string) int {
    return r.queues[queueName].Consumers;
}

func (r *RabbitMQClient) connect() error {
    var err error

    if (r.channel != nil && r.channel.IsClosed()) ||
        (r.conn != nil && r.conn.IsClosed()) {
        return nil
    }
    
    r.conn, err = amqp.Dial(r.url)
    if err != nil {
        return err
    }
    
    r.channel, err = r.conn.Channel()
    if err != nil {
        r.conn.Close()
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

    log.Println("RabbitMQ server shutdown successfully!")
}