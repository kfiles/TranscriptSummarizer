package webhook

import (
	"context"
	"encoding/xml"
	"log"
	"net/http"

	"github.com/kfiles/transcriptsummarizer/pkg/db"
	"github.com/kfiles/transcriptsummarizer/pkg/pipeline"
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
)

// atomFeed is the subset of the YouTube PubSubHubbub Atom payload we care about.
// YouTube sends: <feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" ...>
//
//	<entry><yt:videoId>...</yt:videoId></entry>
//
// </feed>
type atomFeed struct {
	Entry struct {
		VideoID string `xml:"http://www.youtube.com/xml/schemas/2015 videoId"`
		Title   string `xml:"title"`
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
		videoID := feed.Entry.VideoID
		if videoID == "" {
			log.Printf("webhook: empty videoId in feed")
			http.Error(w, "missing videoId", http.StatusBadRequest)
			return
		}
		log.Printf("webhook: received notification for video %s", videoID)

		// Process synchronously — the function timeout (set to 540s) gives us plenty
		// of headroom. Returning an error causes YouTube to retry, which is desirable.
		if err := runPipelineFn(r.Context(), videoID); err != nil {
			log.Printf("webhook: pipeline error for %s: %v", videoID, err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

var runPipelineFn = runPipeline

func runPipeline(ctx context.Context, videoID string) error {
	video, err := transcript.GetVideoByID(videoID)
	if err != nil {
		return err
	}

	facade := db.NewFacade()
	client, err := db.NewClient()
	if err != nil {
		return err
	}
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			log.Printf("webhook: disconnect: %v", err)
		}
	}()

	return pipeline.Run(ctx, facade, client, video)
}
