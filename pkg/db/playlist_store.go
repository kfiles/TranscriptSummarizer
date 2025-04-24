package db

import (
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/net/context"
)

const collectionPlaylists = "playlists"

func (f *dbFacade) ListPlaylists(ctx context.Context, dbClient *mongo.Client, channelId string) ([]*transcript.Playlist, error) {
	return make([]*transcript.Playlist, 0), nil
}

func (f *dbFacade) GetPlaylist(ctx context.Context, dbClient *mongo.Client, playlistID string) (*transcript.Playlist, error) {
	return &transcript.Playlist{}, nil
}

func (f *dbFacade) InsertPlaylist(ctx context.Context, dbClient *mongo.Client, playlist *transcript.Playlist) error {
	_, err := dbClient.Database(databaseName).Collection(collectionPlaylists).InsertOne(ctx, playlist)
	if err != nil {
		return err
	}
	return nil
}

func (f *dbFacade) UpdatePlaylist(ctx context.Context, dbClient *mongo.Client, playlist *transcript.Playlist) error {
	return nil
}

func (f *dbFacade) DeletePlaylist(ctx context.Context, dbClient *mongo.Client, playlistID string) error {
	return nil
}
