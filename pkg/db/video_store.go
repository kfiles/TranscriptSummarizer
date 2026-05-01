package db

import (
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/net/context"
)

const collectionVideos = "videos"

func (f *dbFacade) ListAllVideos(ctx context.Context, dbClient *mongo.Client) ([]*transcript.Video, error) {
	ctx, cancel := capCtx(ctx)
	defer cancel()
	cursor, err := dbClient.Database(databaseName).Collection(collectionVideos).Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())
	var videos []*transcript.Video
	if err := cursor.All(ctx, &videos); err != nil {
		return nil, err
	}
	return videos, nil
}

func (f *dbFacade) ListVideos(ctx context.Context, dbClient *mongo.Client, playlistID string) ([]*transcript.Video, error) {
	return make([]*transcript.Video, 0), nil
}

func (f *dbFacade) GetVideo(ctx context.Context, dbClient *mongo.Client, videoID string) (*transcript.Video, error) {
	ctx, cancel := capCtx(ctx)
	defer cancel()
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
	ctx, cancel := capCtx(ctx)
	defer cancel()
	_, err := dbClient.Database(databaseName).Collection(collectionVideos).InsertOne(ctx, video)
	return err
}

func (f *dbFacade) UpdateVideo(ctx context.Context, dbClient *mongo.Client, video *transcript.Video) error {
	return nil
}

func (f *dbFacade) DeleteVideo(ctx context.Context, dbClient *mongo.Client, videoID string) error {
	return nil
}
