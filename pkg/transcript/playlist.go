package transcript

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

const maxPlaylistResults = 100

type Playlist struct {
	PlaylistId string    `bson:"_id" json:"playlistId"`
	ChannelId  string    `bson:"channelId" json:"channelId"`
	Title      string    `bson:"title" json:"title"`
	UpdatedAt  time.Time `bson:"updatedAt" json:"updatedAt"`
	PageToken  string    `bson:"pageToken" json:"pageToken"`
	NumEntries int64     `bson:"numEntries" json:"numEntries"`
}

// PlaylistEntry pairs a Video with the page token used to retrieve its page.
type PlaylistEntry struct {
	Video     *Video
	PageToken string
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
	service, err := youtube.NewService(context.Background(), option.WithHTTPClient(client))
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

// GetVideoByID fetches metadata for a single video using a YOUTUBE_API_KEY env var.
// This is the auth-free alternative to GetPlaylistItems for use in serverless contexts.
func GetVideoByID(videoID string) (*Video, error) {
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("YOUTUBE_API_KEY environment variable not set")
	}
	service, err := youtube.NewService(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("error creating YouTube client: %w", err)
	}
	response, err := service.Videos.List([]string{"snippet"}).Id(videoID).Do()
	if err != nil {
		return nil, fmt.Errorf("error fetching video %s: %w", videoID, err)
	}
	if len(response.Items) == 0 {
		return nil, fmt.Errorf("video %s not found", videoID)
	}
	item := response.Items[0]
	publishedAt, _ := time.Parse("2006-01-02T15:04:05Z", item.Snippet.PublishedAt)
	return &Video{
		VideoId:     item.Id,
		Title:       item.Snippet.Title,
		Description: item.Snippet.Description,
		PublishedAt: publishedAt,
	}, nil
}

func GetPlaylistItems(playlistId string) ([]*Video, error) {
	client := getClient(youtube.YoutubeReadonlyScope)
	service, err := youtube.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Error creating YouTube client: %v", err)
	}

	var videos []*Video
	pageToken := ""
	for {
		call := service.PlaylistItems.List([]string{"snippet", "contentDetails"})
		call = call.PlaylistId(playlistId)
		call = call.MaxResults(maxPlaylistResults)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		response, perr := call.Do()
		if perr != nil {
			return nil, fmt.Errorf("error getting playlist items: %v", perr)
		}
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
		if response.NextPageToken == "" {
			break
		}
		pageToken = response.NextPageToken
	}
	return videos, nil
}

// GetChannelPlaylists fetches all playlists for a channel using YOUTUBE_API_KEY,
// requesting 50 results per page and following pagination until exhausted.
func GetChannelPlaylists(channelId string) ([]*Playlist, error) {
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("YOUTUBE_API_KEY environment variable not set")
	}
	service, err := youtube.NewService(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("create YouTube client: %w", err)
	}

	var playlists []*Playlist
	pageToken := ""
	for {
		call := service.Playlists.List([]string{"snippet", "contentDetails"}).
			ChannelId(channelId).
			MaxResults(50)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("playlists.list: %w", err)
		}
		for _, item := range resp.Items {
			playlists = append(playlists, &Playlist{
				PlaylistId: item.Id,
				ChannelId:  channelId,
				Title:      item.Snippet.Title,
				NumEntries: item.ContentDetails.ItemCount,
			})
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return playlists, nil
}

// fetchPage is the signature of the inner API call, injectable for tests.
type fetchPage func(playlistId, pageToken string, pageSize int64) (items []*youtube.PlaylistItem, nextToken string, err error)

// newPlaylistFetcher is injectable for tests.
var newPlaylistFetcher = defaultPlaylistFetcher

func defaultPlaylistFetcher(apiKey string) fetchPage {
	service, err := youtube.NewService(context.Background(), option.WithAPIKey(apiKey))
	if err != nil {
		return func(_, _ string, _ int64) ([]*youtube.PlaylistItem, string, error) {
			return nil, "", fmt.Errorf("create YouTube client: %w", err)
		}
	}
	return func(playlistId, pageToken string, pageSize int64) ([]*youtube.PlaylistItem, string, error) {
		call := service.PlaylistItems.List([]string{"snippet"}).
			PlaylistId(playlistId).
			MaxResults(pageSize)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return nil, "", err
		}
		return resp.Items, resp.NextPageToken, nil
	}
}

// ScanPlaylist fetches all playlist items starting from startPageToken, pageSize
// items per request, and returns them sorted by PublishedAt ascending.
// Uses YOUTUBE_API_KEY for auth (no OAuth required).
func ScanPlaylist(playlistId, startPageToken string, pageSize int64) ([]*PlaylistEntry, error) {
	apiKey := os.Getenv("YOUTUBE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("YOUTUBE_API_KEY environment variable not set")
	}
	fetch := newPlaylistFetcher(apiKey)

	var entries []*PlaylistEntry
	pageToken := startPageToken
	for {
		items, nextToken, err := fetch(playlistId, pageToken, pageSize)
		if err != nil {
			return nil, fmt.Errorf("playlistItems.list: %w", err)
		}
		for _, item := range items {
			publishedAt, _ := time.Parse("2006-01-02T15:04:05Z", item.Snippet.PublishedAt)
			entries = append(entries, &PlaylistEntry{
				Video: &Video{
					VideoId:     item.Snippet.ResourceId.VideoId,
					PlaylistId:  item.Snippet.PlaylistId,
					Title:       item.Snippet.Title,
					Description: item.Snippet.Description,
					Position:    item.Snippet.Position,
					PublishedAt: publishedAt,
				},
				PageToken: pageToken,
			})
		}
		if nextToken == "" {
			break
		}
		pageToken = nextToken
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Video.PublishedAt.Before(entries[j].Video.PublishedAt)
	})
	return entries, nil
}
