package mongodb

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"

	db "TaskOneUtils/db"
	ent "TaskOneUtils/db/entities"
)

type MongoDBClient struct {
	client *mongo.Client
	database *mongo.Database
    stop chan os.Signal
}

func signalsSetup() chan os.Signal {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	return stop
}

func (m *MongoDBClient) waitReconnect() chan os.Signal {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    arrived := make(chan os.Signal, 1)
    for {
        ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
        err := m.client.Ping(ctx, nil)
        cancel()

        if err == nil {
            log.Println("Reconnect to MongoDB successfully")
            return arrived
        }

        log.Println("Reconnection to MongoDB failed, retry in 5 seconds...")

        select {
        case <- ticker.C:
            continue
        case s := <- m.stop:
            log.Println("Stopping signal requested")
            arrived <- s
            return arrived
        }
    }
}

func (m *MongoDBClient) tryExecute(operation func() error) error {
    for {
        if m.client != nil {
            err := operation()
            if err == nil {
                return nil
            }
            
            if err != mongo.ErrClientDisconnected {
                return err
            }
            
            log.Printf("Connection error during operation: %v, will retry", err)

            select {
            case <- m.waitReconnect():
                return nil
            case <- m.stop:
                log.Println("Stopping signal requested")
                return nil
            }
        }
    }
}

func NewClient(uri string, dbName string) (db.Database, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

	clientOpts := options.Client().
						ApplyURI(uri).
						SetReadConcern(readconcern.Majority()).
						SetReadPreference(readpref.Nearest()).
						SetWriteConcern(writeconcern.Majority())
	
	client, err := mongo.Connect(clientOpts)
	if err != nil {
		log.Printf("Can not create MongoDB client %s\n", err)
		return nil, err;
	}

	err = client.Ping(ctx, nil);
	if err != nil {
		log.Printf("Can not establish MongoDB connection %s\n", err)
		return nil, err;
	}

	return &MongoDBClient{
        client:   client,
        database: client.Database(dbName),
    }, nil
}

func (m *MongoDBClient) CreateRequestWithoutConnRetry(req *ent.Request) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    req.CreatedAt = time.Now()
    req.UpdatedAt = time.Now()
    
    collection := m.database.Collection("requests")
    result, err := collection.InsertOne(ctx, req)

    if err != nil {
        log.Printf("Can not insert entity to MongoDB %s\n", err)
        return err
    }

    req.ID = result.InsertedID.(bson.ObjectID)
    
    return nil
}

func (m *MongoDBClient) CreateRequest(req *ent.Request) error {
    err := m.tryExecute(func() error {
        return m.CreateRequestWithoutConnRetry(req)
    })

    if err != nil {
        log.Fatalf("Insertion failure %s", err)
    }
    return nil
}

func (m *MongoDBClient) GetRequestByID(requestID string) (*ent.Request, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    var req ent.Request
    collection := m.database.Collection("requests")
    filter := bson.M{"request_id": requestID}

    err := m.tryExecute(func() error {
        err := collection.FindOne(ctx, filter).Decode(&req)

        if err != nil {
            if err == mongo.ErrNoDocuments {
                return nil
            }
            return err
        }
        
        return nil
    })  

    if err != nil {
        log.Fatalf("Reading failure %s", err)
    }
    
    return &req, nil
}

func (m *MongoDBClient) GetAllRequests() ([]ent.Request, error) {
    ctx := context.Background()
    
    var requests []ent.Request
    collection := m.database.Collection("requests")

    err := m.tryExecute(func() error {
        cursor, err := collection.Find(ctx, bson.M{})
        if err != nil {
            if err == mongo.ErrNoDocuments {
                return nil
            }
            return err
        }

        defer cursor.Close(ctx)

        if err = cursor.All(ctx, &requests); err != nil {
            return err
        }

        return nil
    })  

    if err != nil {
        log.Fatalf("Reading all failure %s", err)
    }
    
    return requests, nil
}

func (m *MongoDBClient) UpdateRequestReceivedPartsAndWords(requestID string, receivedPart int, words []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if words == nil {
        words = []string{}
    }
    
    collection := m.database.Collection("requests")
    
    filter := bson.M{"request_id": requestID}
    update := bson.M{
        "$set": bson.M{
            "received_parts." + strconv.Itoa(receivedPart): true,
            "updated_at": time.Now(),
        },
        "$push": bson.M{
            "words": bson.M{"$each": words},
        },
    }

    err := m.tryExecute(func() error {
        result, err := collection.UpdateOne(ctx, filter, update)
        if err != nil {
            return err
        }
        
        if result.MatchedCount == 0 {
            return fmt.Errorf("Request for update not found")
        }

        return nil
    })  

    if err != nil {
        log.Fatalf("Update failure %s", err)
    }
    
    return nil
}

func (m *MongoDBClient) DeleteRequest(id string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    collection := m.database.Collection("requests")
	filter := bson.M{"request_id": id}

    err := m.tryExecute(func() error {
        result, err := collection.DeleteOne(ctx, filter)

        if err != nil || result.DeletedCount != 1 {
            log.Printf("Can not delete entity from MongoDB %s\n", err)
            return err
        }

        return nil
    })  

    if err != nil {
        log.Fatalf("Delete failure %s", err)
    }

    return nil
}

func (m *MongoDBClient) Close() error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    return m.client.Disconnect(ctx)
}