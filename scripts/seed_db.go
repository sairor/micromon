package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(ctx)

	db := client.Database("mikromon")
	fmt.Println("Connected to MongoDB")

	// 1. Users
	createUsers(ctx, db)

	// 2. Devices (Equipamentos)
	createDevices(ctx, db)

	// 3. Custom Commands
	createCommands(ctx, db)

	// 4. Signal History Indexes
	createSignalIndexes(ctx, db)

	fmt.Println("Seeding completed successfully.")
}

func createUsers(ctx context.Context, db *mongo.Database) {
	coll := db.Collection("users")

	// Index on username
	_, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		log.Println("Index error users:", err)
	}

	// Seed Admin
	hash, _ := bcrypt.GenerateFromPassword([]byte("admin"), bcrypt.DefaultCost)
	admin := bson.M{"username": "admin", "password_hash": string(hash), "role": "admin"}

	opts := options.Update().SetUpsert(true)
	_, err = coll.UpdateOne(ctx, bson.M{"username": "admin"}, bson.M{"$set": admin}, opts)
	if err != nil {
		log.Println("Error seeding admin:", err)
	}
	fmt.Println("Seeded Users")
}

func createDevices(ctx context.Context, db *mongo.Database) {
	coll := db.Collection("devices")
	// Index IP
	coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "ip", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	fmt.Println("Configured Devices Collection")
}

func createCommands(ctx context.Context, db *mongo.Database) {
	// Just ensure collection exists logic if needed, but Mongo creates on write.
	// We can explicitly create collection checking names, but for seed simple is fine.
	fmt.Println("Configured Custom Commands Collection")
}

func createSignalIndexes(ctx context.Context, db *mongo.Database) {
	coll := db.Collection("historico_sinal")

	// Index: rx_power for sorting
	coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "rx_power", Value: 1}},
	})

	// Index: device + onu + timestamp
	coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "device_name", Value: 1},
			{Key: "onu_serial", Value: 1},
			{Key: "timestamp", Value: -1},
		},
	})

	// TTL Index (retain logs for 30 days)
	// Assuming 'timestamp' is a Date object, if int64 we can't use TTL easily without a shadow date field.
	// For now skipping TTL on int64.

	fmt.Println("Configured Signal History Indexes")
}
