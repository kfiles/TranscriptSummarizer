package db

import (
	"context"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const databaseName = "meetingtranscripts"

// maxOperationTimeout caps maxTimeMS sent to Firestore's MongoDB compatibility
// layer, which rejects values above 60000ms. SetTimeout on the client does not
// help because the driver uses the context deadline directly when one is present;
// capping must happen at the call site.
const maxOperationTimeout = 55 * time.Second

// capCtx returns a child context whose deadline is at most maxOperationTimeout
// from now, regardless of how long the parent context lives.
func capCtx(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, maxOperationTimeout)
}

func NewClient() (*mongo.Client, error) {
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		log.Fatal("Set your 'MONGODB_URI' environment variable.")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		log.Fatal("Error connecting to MongoDB: ", err)
	}
	return client, nil
}
