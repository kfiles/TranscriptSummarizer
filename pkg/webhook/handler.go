package webhook

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/kfiles/transcriptsummarizer/pkg/db"
	"github.com/kfiles/transcriptsummarizer/pkg/pipeline"
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
)

// atomFeed is the subset of the YouTube PubSubHubbub Atom payload we care about.
type atomFeed struct {
	Entry struct {
		VideoID   string `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
		ChannelID string `xml:"http://www.youtube.com/xml/schemas/2015 channelId"`
		Title     string `xml:"title"`
	} `xml:"entry"`
}

// Handler is the HTTP handler for YouTube PubSubHubbub notifications.
// Exported so the root package can expose it as the Cloud Functions entry point.
func Handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// YouTube subscription verification: echo hub.challenge back verbatim.
		challenge := r.URL.Query().Get("hub.challenge")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(challenge)) //nolint:errcheck

	case http.MethodPost:
		var feed atomFeed
		if err := xml.NewDecoder(r.Body).Decode(&feed); err != nil {
			log.Printf("webhook: parse atom feed: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		channelID := feed.Entry.ChannelID
		if channelID == "" {
			log.Printf("webhook: empty channelId in feed")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		log.Printf("webhook: received notification for channel %s (video %s)", channelID, feed.Entry.VideoID)

		if err := runPipelineFn(r.Context(), channelID); err != nil {
			log.Printf("webhook: processing error for channel %s: %v", channelID, err)
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

var (
	runPipelineFn       = runPipeline
	newFacadeFn         = func() db.Facade { return db.NewFacade() }
	newDBClientFn       = db.NewClient
	scanPlaylistFn      = transcript.ScanPlaylist
	runVideoPipelineFn  = pipeline.Run
	writeAllMarkdownFn  = pipeline.WriteAllMarkdown
)

func runPipeline(ctx context.Context, channelID string) error {
	facade := newFacadeFn()
	client, err := newDBClientFn()
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer func() {
		if client != nil {
			if err := client.Disconnect(ctx); err != nil {
				log.Printf("webhook: disconnect: %v", err)
			}
		}
	}()

	playlists, err := facade.ListPlaylists(ctx, client, channelID)
	if err != nil {
		log.Printf("webhook: list playlists for channel %s: %v", channelID, err)
		return nil
	}
	if len(playlists) == 0 {
		log.Printf("webhook: no playlists found for channel %s", channelID)
		return nil
	}

	pageSize := envInt64("PLAYLIST_PAGE_SIZE", 50)
	threshold := envInt64("PLAYLIST_SCAN_THRESHOLD", 100)
	maxFailures := envInt("MAX_PIPELINE_FAILURES", 3)

	failCount := 0
	successCount := 0
	circuitTripped := false

	for _, pl := range playlists {
		if circuitTripped {
			break
		}

		startToken := ""
		if pl.NumEntries >= threshold {
			startToken = pl.PageToken
		}

		log.Printf("webhook: scanning playlist %s (%s) from token %q", pl.PlaylistId, pl.Title, startToken)
		entries, err := scanPlaylistFn(pl.PlaylistId, startToken, pageSize)
		if err != nil {
			log.Printf("webhook: scan playlist %s: %v", pl.PlaylistId, err)
			continue
		}
		log.Printf("webhook: playlist %s returned %d entries", pl.PlaylistId, len(entries))

		for _, entry := range entries {
			if _, err := facade.GetVideo(ctx, client, entry.Video.VideoId); err == nil {
				continue // already processed
			}

			if err := runVideoPipelineFn(ctx, facade, client, entry.Video); err != nil {
				log.Printf("webhook: pipeline error for video %s: %v", entry.Video.VideoId, err)
				failCount++
				if failCount >= maxFailures {
					log.Printf("webhook: circuit breaker tripped after %d failures", failCount)
					circuitTripped = true
					break
				}
				continue
			}
			successCount++

			// Update playlist record with the page where this video was found.
			if entry.Video.Position+1 > pl.NumEntries {
				pl.NumEntries = entry.Video.Position + 1
			}
			pl.PageToken = entry.PageToken
			pl.UpdatedAt = time.Now()
			if err := facade.UpdatePlaylist(ctx, client, pl); err != nil {
				log.Printf("webhook: update playlist %s: %v", pl.PlaylistId, err)
			}
		}
	}

	if successCount > 0 {
		log.Printf("webhook: %d new transcript(s) processed — regenerating Hugo content", successCount)
		if err := writeAllMarkdownFn(ctx, facade, client); err != nil {
			log.Printf("webhook: write all markdown: %v", err)
		}
	}

	return nil // always swallow; YouTube should not retry
}

func envInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func envInt64(key string, defaultVal int64) int64 {
	return int64(envInt(key, int(defaultVal)))
}
