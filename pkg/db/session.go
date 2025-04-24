package db

import (
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"log"
	"os"
)

const databaseName = "meetingtranscripts"

func NewClient() (*mongo.Client, error) {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		log.Fatal("Set your 'MONGODB_URI' environment variable.")
	}
	client, err := mongo.Connect(options.Client().
		ApplyURI(uri))
	if err != nil {
		log.Fatal("Error connecting to MongoDB: ", err)
	}
	return client, nil
}
