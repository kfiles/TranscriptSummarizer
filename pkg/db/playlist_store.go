package db

import (
	"context"
	"fmt"

	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const collectionPlaylists = "playlists"

func (f *dbFacade) ListPlaylists(ctx context.Context, dbClient *mongo.Client, channelId string) ([]*transcript.Playlist, error) {
	ctx, cancel := capCtx(ctx)
	defer cancel()

	filter := bson.D{}
	if channelId != "" {
		filter = bson.D{{Key: "channelId", Value: channelId}}
	}

	cur, err := dbClient.Database(databaseName).Collection(collectionPlaylists).Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("playlists: find: %w", err)
	}
	defer cur.Close(ctx)

	var playlists []*transcript.Playlist
	for cur.Next(ctx) {
		var p transcript.Playlist
		if err := cur.Decode(&p); err != nil {
			return nil, fmt.Errorf("playlists: decode: %w", err)
		}
		playlists = append(playlists, &p)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("playlists: cursor: %w", err)
	}
	return playlists, nil
}

func (f *dbFacade) GetPlaylist(ctx context.Context, dbClient *mongo.Client, playlistID string) (*transcript.Playlist, error) {
	ctx, cancel := capCtx(ctx)
	defer cancel()

	res := dbClient.Database(databaseName).Collection(collectionPlaylists).FindOne(ctx, bson.D{{Key: "_id", Value: playlistID}})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var p transcript.Playlist
	if err := res.Decode(&p); err != nil {
		return nil, fmt.Errorf("playlists: decode: %w", err)
	}
	return &p, nil
}

func (f *dbFacade) InsertPlaylist(ctx context.Context, dbClient *mongo.Client, playlist *transcript.Playlist) error {
	ctx, cancel := capCtx(ctx)
	defer cancel()

	_, err := dbClient.Database(databaseName).Collection(collectionPlaylists).InsertOne(ctx, playlist)
	return err
}

func (f *dbFacade) UpdatePlaylist(ctx context.Context, dbClient *mongo.Client, playlist *transcript.Playlist) error {
	ctx, cancel := capCtx(ctx)
	defer cancel()

	filter := bson.D{{Key: "_id", Value: playlist.PlaylistId}}
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "pageToken", Value: playlist.PageToken},
		{Key: "numEntries", Value: playlist.NumEntries},
		{Key: "updatedAt", Value: playlist.UpdatedAt},
	}}}
	_, err := dbClient.Database(databaseName).Collection(collectionPlaylists).UpdateOne(ctx, filter, update)
	return err
}

func (f *dbFacade) UpsertPlaylist(ctx context.Context, dbClient *mongo.Client, playlist *transcript.Playlist) error {
	ctx, cancel := capCtx(ctx)
	defer cancel()

	filter := bson.D{{Key: "_id", Value: playlist.PlaylistId}}
	update := bson.D{{Key: "$set", Value: playlist}}
	_, err := dbClient.Database(databaseName).Collection(collectionPlaylists).UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
	return err
}

func (f *dbFacade) DeletePlaylist(ctx context.Context, dbClient *mongo.Client, playlistID string) error {
	ctx, cancel := capCtx(ctx)
	defer cancel()

	_, err := dbClient.Database(databaseName).Collection(collectionPlaylists).DeleteOne(ctx, bson.D{{Key: "_id", Value: playlistID}})
	return err
}
