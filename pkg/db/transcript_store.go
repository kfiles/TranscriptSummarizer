package db

import (
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/net/context"
)

const collectionTranscripts = "transcripts"

func (f *dbFacade) ListTranscripts(ctx context.Context, dbClient *mongo.Client, videoId string) ([]*transcript.Transcript, error) {
	return make([]*transcript.Transcript, 0), nil
}

func (f *dbFacade) GetTranscript(ctx context.Context, dbClient *mongo.Client, videoID string, languageCode string) (*transcript.Transcript, error) {
	res := dbClient.Database(databaseName).Collection(collectionTranscripts).FindOne(ctx,
		bson.D{{"videoId", videoID}, {"languageCode", languageCode}})
	if res.Err() == mongo.ErrNoDocuments {
		return nil, res.Err()
	}
	t := &transcript.Transcript{}
	err := res.Decode(t)
	return t, err
}

func (f *dbFacade) InsertTranscript(ctx context.Context, dbClient *mongo.Client, transcript *transcript.Transcript) error {
	_, err := dbClient.Database(databaseName).Collection(collectionTranscripts).InsertOne(ctx, transcript)
	return err
}

func (f *dbFacade) UpdateTranscript(ctx context.Context, dbClient *mongo.Client, transcript *transcript.Transcript) error {
	filter := bson.D{{"videoId", transcript.VideoId}, {"languageCode", transcript.LanguageCode}}
	update := bson.D{{"$set", bson.D{{"summaryText", transcript.SummaryText}}}}
	_, err := dbClient.Database(databaseName).Collection(collectionTranscripts).UpdateOne(ctx, filter, update)
	return err
}

func (f *dbFacade) DeleteTranscript(ctx context.Context, dbClient *mongo.Client, videoID string) error {
	return nil
}
