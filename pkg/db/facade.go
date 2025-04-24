package db

import (
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
	"golang.org/x/net/context"
)
import "go.mongodb.org/mongo-driver/v2/mongo"

type Facade interface {
	ListPlaylists(ctx context.Context, dbClient *mongo.Client, channelId string) ([]*transcript.Playlist, error)
	GetPlaylist(ctx context.Context, dbClient *mongo.Client, playlistID string) (*transcript.Playlist, error)
	InsertPlaylist(ctx context.Context, dbClient *mongo.Client, playlist *transcript.Playlist) error
	UpdatePlaylist(ctx context.Context, dbClient *mongo.Client, playlist *transcript.Playlist) error
	DeletePlaylist(ctx context.Context, dbClient *mongo.Client, playlistID string) error
	ListVideos(ctx context.Context, dbClient *mongo.Client, playlistID string) ([]*transcript.Video, error)
	GetVideo(ctx context.Context, dbClient *mongo.Client, videoID string) (*transcript.Video, error)
	InsertVideo(ctx context.Context, dbClient *mongo.Client, video *transcript.Video) error
	UpdateVideo(ctx context.Context, dbClient *mongo.Client, video *transcript.Video) error
	DeleteVideo(ctx context.Context, dbClient *mongo.Client, videoID string) error
	ListTranscripts(ctx context.Context, dbClient *mongo.Client, videoId string) ([]*transcript.Transcript, error)
	GetTranscript(ctx context.Context, dbClient *mongo.Client, videoID string, languageCode string) (*transcript.Transcript, error)
	InsertTranscript(ctx context.Context, dbClient *mongo.Client, transcript *transcript.Transcript) error
	UpdateTranscript(ctx context.Context, dbClient *mongo.Client, transcript *transcript.Transcript) error
	DeleteTranscript(ctx context.Context, dbClient *mongo.Client, videoID string) error
}

type dbFacade struct{}

func NewFacade() Facade {
	return &dbFacade{}
}
