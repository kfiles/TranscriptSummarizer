package db

import (
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/net/context"
)

const collectionTranscripts = "transcripts"

func (f *dbFacade) ListTranscripts(ctx context.Context, dbClient *mongo.Client, videoId string) ([]*transcript.Transcript, error) {
	ctx, cancel := capCtx(ctx)
	defer cancel()
	cursor, err := dbClient.Database(databaseName).Collection(collectionTranscripts).Find(ctx, bson.D{{"videoId", videoId}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	var transcripts []*transcript.Transcript
	if err := cursor.All(ctx, &transcripts); err != nil {
		return nil, err
	}
	return transcripts, nil
}

func (f *dbFacade) GetTranscript(ctx context.Context, dbClient *mongo.Client, videoID string, languageCode string) (*transcript.Transcript, error) {
	ctx, cancel := capCtx(ctx)
	defer cancel()
	res := dbClient.Database(databaseName).Collection(collectionTranscripts).FindOne(ctx,
		bson.D{{"videoId", videoID}, {"languageCode", languageCode}})
	if res.Err() == mongo.ErrNoDocuments {
		return nil, res.Err()
	}
	t := &transcript.Transcript{}
	err := res.Decode(t)
	return t, err
}

func (f *dbFacade) InsertTranscript(ctx context.Context, dbClient *mongo.Client, t *transcript.Transcript) error {
	ctx, cancel := capCtx(ctx)
	defer cancel()
	_, err := dbClient.Database(databaseName).Collection(collectionTranscripts).InsertOne(ctx, t)
	return err
}

func (f *dbFacade) UpdateTranscript(ctx context.Context, dbClient *mongo.Client, t *transcript.Transcript) error {
	ctx, cancel := capCtx(ctx)
	defer cancel()
	filter := bson.D{{"videoId", t.VideoId}, {"languageCode", t.LanguageCode}}
	update := bson.D{{"$set", bson.D{{"summaryText", t.SummaryText}}}}
	_, err := dbClient.Database(databaseName).Collection(collectionTranscripts).UpdateOne(ctx, filter, update)
	return err
}

func (f *dbFacade) DeleteTranscript(ctx context.Context, dbClient *mongo.Client, videoID string) error {
	return nil
}
