package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/kfiles/transcriptsummarizer/pkg/officials"
)

const (
	databaseName        = "meetingtranscripts"
	collectionName      = "officials"
	maxOperationTimeout = 55 * time.Second
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// --- Scrape ---
	html, err := officials.Fetch(ctx, nil)
	if err != nil {
		log.Fatalf("fetch: %v", err)
	}
	committees, err := officials.ParseTownWideDOM(strings.NewReader(html))
	if err != nil {
		log.Fatalf("parse: %v", err)
	}

	// --- Print ---
	for _, c := range committees {
		fmt.Printf("\n%s\n", c.Name)
		for _, m := range c.Members {
			fmt.Printf("  %s\n", m)
		}
	}

	// --- Persist ---
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Println("MONGODB_URI not set; skipping database write")
		return
	}

	client, err := mongo.Connect(options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatalf("mongo connect: %v", err)
	}
	defer func() {
		if err := client.Disconnect(context.Background()); err != nil {
			log.Printf("mongo disconnect: %v", err)
		}
	}()

	col := client.Database(databaseName).Collection(collectionName)
	n, err := replaceCommittees(ctx, col, committees)
	if err != nil {
		log.Fatalf("write: %v", err)
	}
	fmt.Printf("\n%d committees written to %s.%s\n", n, databaseName, collectionName)
}

// replaceCommittees drops the officials collection and re-inserts every
// committee as a fresh document. Drop-then-insert is used instead of upsert
// because Firestore's MongoDB compatibility layer does not support BulkWrite.
func replaceCommittees(ctx context.Context, col *mongo.Collection, committees []officials.Committee) (int, error) {
	writeCtx, cancel := context.WithTimeout(ctx, maxOperationTimeout)
	defer cancel()

	if err := col.Drop(writeCtx); err != nil {
		return 0, fmt.Errorf("drop collection: %w", err)
	}

	docs := make([]interface{}, 0, len(committees))
	for _, c := range committees {
		docs = append(docs, bson.D{
			{Key: "_id", Value: c.Name},
			{Key: "name", Value: c.Name},
			{Key: "members", Value: c.Members},
		})
	}

	if _, err := col.InsertMany(writeCtx, docs); err != nil {
		return 0, fmt.Errorf("insert: %w", err)
	}
	return len(docs), nil
}
