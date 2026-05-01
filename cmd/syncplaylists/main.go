package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/kfiles/transcriptsummarizer/pkg/db"
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
)

// targetPlaylistTitles defines the subset of channel playlists to sync to MongoDB.
var targetPlaylistTitles = []string{
	"Select Board 2026",
	"Planning Board 2026",
	"Conservation Commission 2026",
	"School Committee 2026",
	"Warrant Committee 2026",
}

func main() {
	channelId := flag.String("channelId", "", "YouTube channel ID to fetch playlists for")
	flag.Parse()
	if *channelId == "" {
		log.Fatal("channelId flag is required")
	}

	playlists, err := transcript.GetChannelPlaylists(*channelId)
	if err != nil {
		log.Fatalf("fetch playlists: %v", err)
	}
	fmt.Printf("Retrieved %d playlists from channel %s\n", len(playlists), *channelId)

	targetSet := make(map[string]bool, len(targetPlaylistTitles))
	for _, t := range targetPlaylistTitles {
		targetSet[t] = true
	}

	var matched []*transcript.Playlist
	for _, p := range playlists {
		if targetSet[p.Title] {
			matched = append(matched, p)
		}
	}
	fmt.Printf("Matched %d/%d target playlists\n", len(matched), len(targetPlaylistTitles))

	facade := db.NewFacade()
	client, err := db.NewClient()
	if err != nil {
		log.Fatalf("connect to MongoDB: %v", err)
	}
	defer func() {
		if derr := client.Disconnect(context.TODO()); derr != nil {
			log.Printf("disconnect MongoDB: %v", derr)
		}
	}()

	ctx := context.Background()
	for _, p := range matched {
		if uerr := facade.UpsertPlaylist(ctx, client, p); uerr != nil {
			log.Printf("upsert playlist %q (%s): %v", p.Title, p.PlaylistId, uerr)
			continue
		}
		fmt.Printf("Stored: %s (%s)\n", p.Title, p.PlaylistId)
	}
}
