package mongodb

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readconcern"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"

	ent "TaskOneUtils/db/entities"
)

type MongoDBClient struct {
	client *mongo.Client
	database *mongo.Database
}

func NewClient(uri string, dbName string) (*MongoDBClient, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

	clientOpts := options.Client().
						ApplyURI(uri).
						SetReadConcern(readconcern.Majority()).
						SetReadPreference(readpref.Nearest()).
						SetWriteConcern(writeconcern.Majority())
	
	client, err := mongo.Connect(clientOpts)
	if err != nil {
		log.Panicf("Can not create MongoDB client %s\n", err)
		return nil, err;
	}

	err = client.Ping(ctx, nil);
	if err != nil {
		log.Panicf("Can not establish MongoDB connection %s\n", err)
		return nil, err;
	}

	return &MongoDBClient{
        client:   client,
        database: client.Database(dbName),
    }, nil
}

func (m *MongoDBClient) CreateRequest(req *ent.Request) error {
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
    
    req.ID = result.InsertedID.(primitive.ObjectID)
    return nil
}

func (m *MongoDBClient) GetRequestByID(requestID string) (*ent.Request, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    var req ent.Request
    collection := m.database.Collection("requests")
    
    filter := bson.M{"request_id": requestID}
    err := collection.FindOne(ctx, filter).Decode(&req)
    if err != nil {
        if err == mongo.ErrNoDocuments {
            return nil, nil
        }
        return nil, err
    }
    
    return &req, nil
}

func (m *MongoDBClient) DeleteRequest(id string) error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    collection := m.database.Collection("requests")

	filter := bson.M{"request_id": id}
    result, err := collection.DeleteOne(ctx, filter)
    if err != nil || result.DeletedCount != 1 {
		log.Printf("Can not insert entity to MongoDB %s\n", err)
        return err
    }

    return nil
}

func (m *MongoDBClient) Close() error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    return m.client.Disconnect(ctx)
}