package transcript

import (
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/api/option"
	"log"
	"time"

	"google.golang.org/api/youtube/v3"
)

const maxPlaylistResults = 100

type Playlist struct {
	PlaylistId string `bson:"_id" json:"playlistId"`
	ChannelId  string `bson:"channelId" json:"channelId"`
	Title      string `bson:"title" json:"title"`
}

type Video struct {
	VideoId     string    `bson:"_id" json:"videoId"`
	PlaylistId  string    `bson:"playlistId" json:"playlistId"`
	Title       string    `bson:"title" json:"title"`
	Description string    `bson:"description" json:"description"`
	Position    int64     `bson:"position" json:"position"`
	PublishedAt time.Time `bson:"publishedAt" json:"publishedAt"`
}

func playlistsList(service *youtube.Service, part []string, channelId string, hl string, maxResults int64, playlistId string, pageToken string) (*youtube.PlaylistListResponse, error) {
	call := service.Playlists.List(part)
	if channelId != "" {
		if playlistId != "" {
			return nil, fmt.Errorf("invalid arguments: specify one of channelId or playlistId")
		}
		call = call.ChannelId(channelId)
	}
	call = call.MaxResults(maxResults)

	if pageToken != "" {
		call = call.PageToken(pageToken)
	}
	if playlistId != "" {
		call = call.Id(playlistId)
	}
	response, err := call.Do()
	if err != nil {
		log.Printf("Error getting playlist list: %v", err)
		return nil, err
	}
	return response, nil
}

func GetPlaylists(channelId string, playlistId string, pageToken string) ([]*Playlist, error) {
	client := getClient(youtube.YoutubeReadonlyScope)
	ctx := context.Background()
	service, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Error creating YouTube client: %v", err)
	}

	response, err := playlistsList(service, []string{"snippet", "contentDetails"}, channelId, "", maxPlaylistResults, playlistId, pageToken)
	if err != nil {
		return nil, err
	}
	playlists := make([]*Playlist, 0, len(response.Items))
	for _, item := range response.Items {
		playlists = append(playlists, &Playlist{
			PlaylistId: item.Id,
			ChannelId:  channelId,
			Title:      item.Snippet.Title,
		})
	}
	return playlists, nil
}

func GetPlaylistItems(playlistId string, pageToken string) ([]*Video, error) {
	client := getClient(youtube.YoutubeReadonlyScope)
	ctx := context.Background()
	service, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Error creating YouTube client: %v", err)
	}
	call := service.PlaylistItems.List([]string{"snippet", "contentDetails"})
	call = call.PlaylistId(playlistId)
	call = call.MaxResults(maxPlaylistResults)
	if pageToken != "" {
		call.PageToken(pageToken)
	}
	response, perr := call.Do()
	if perr != nil {
		return nil, fmt.Errorf("error getting playlist items: %v", perr)
	}
	videos := make([]*Video, 0, len(response.Items))
	for _, item := range response.Items {
		publishedAt, terr := time.Parse("2006-01-02T15:04:05Z", item.Snippet.PublishedAt)
		if terr != nil {
			publishedAt = time.Time{}
		}
		videos = append(videos, &Video{
			VideoId:     item.Snippet.ResourceId.VideoId,
			PlaylistId:  item.Snippet.PlaylistId,
			Title:       item.Snippet.Title,
			Description: item.Snippet.Description,
			Position:    item.Snippet.Position,
			PublishedAt: publishedAt,
		})
	}
	return videos, nil
}
