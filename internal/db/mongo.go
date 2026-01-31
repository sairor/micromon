package db

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Client *mongo.Client

// InitMongoDB initializes the MongoDB connection
func InitMongoDB(uri string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect
	clientOptions := options.Client().ApplyURI(uri)
	var err error
	Client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
        log.Println("WARNING: MongoDB driver init failed. Running in MOCK MODE.")
        Client = nil
		return nil // Proceed without DB
	}

	// Ping the database
	err = Client.Ping(ctx, nil)
	if err != nil {
        log.Println("WARNING: MongoDB connection failed (Ping). Running in MOCK MODE.")
        Client = nil
        return nil
	}

	log.Println("Connected to MongoDB successfully")
	return nil
}

// GetCollection returns a handle to a MongoDB collection
func GetCollection(collectionName string) *mongo.Collection {
    if Client == nil {
        return nil
    }
	return Client.Database("mikromon").Collection(collectionName)
}
