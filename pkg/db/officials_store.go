package db

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const collectionOfficials = "officials"

// ListOfficialNames returns every member name stored across all committees in
// the officials collection. Names are returned in the order they appear in the
// database. The caller should treat an empty slice as a soft failure and fall
// back to calling Summarize without a names list rather than hard-failing.
func ListOfficialNames(ctx context.Context, client *mongo.Client) ([]string, error) {
	ctx, cancel := capCtx(ctx)
	defer cancel()

	cur, err := client.Database(databaseName).Collection(collectionOfficials).Find(ctx, bson.D{})
	if err != nil {
		return nil, fmt.Errorf("officials: find: %w", err)
	}
	defer cur.Close(ctx)

	var names []string
	for cur.Next(ctx) {
		var doc struct {
			Members []string `bson:"members"`
		}
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("officials: decode: %w", err)
		}
		names = append(names, doc.Members...)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("officials: cursor: %w", err)
	}
	return names, nil
}
