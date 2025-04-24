package db

import (
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/net/context"
)

const collectionVideos = "videos"

func (f *dbFacade) ListVideos(ctx context.Context, dbClient *mongo.Client, playlistID string) ([]*transcript.Video, error) {
	return make([]*transcript.Video, 0), nil
}

func (f *dbFacade) GetVideo(ctx context.Context, dbClient *mongo.Client, videoID string) (*transcript.Video, error) {
	res := dbClient.Database(databaseName).Collection(collectionVideos).FindOne(ctx, bson.D{{"_id", videoID}})
	if res.Err() == mongo.ErrNoDocuments {
		return nil, res.Err()
	}
	v := &transcript.Video{}
	err := res.Decode(v)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func (f *dbFacade) InsertVideo(ctx context.Context, dbClient *mongo.Client, video *transcript.Video) error {
	_, err := dbClient.Database(databaseName).Collection(collectionVideos).InsertOne(ctx, video)
	if err != nil {
		return err
	}
	return nil
}

func (f *dbFacade) UpdateVideo(ctx context.Context, dbClient *mongo.Client, video *transcript.Video) error {
	return nil
}

func (f *dbFacade) DeleteVideo(ctx context.Context, dbClient *mongo.Client, videoID string) error {
	return nil
}
