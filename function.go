// Package transcriptsummarizer is the Cloud Functions entry point.
// The buildpack calls YouTubeWebhook by name as an exported function.
package transcriptsummarizer

import (
	"net/http"

	"github.com/kfiles/transcriptsummarizer/pkg/webhook"
)

// YouTubeWebhook is the HTTP Cloud Function entry point for YouTube
// PubSubHubbub push notifications.
func YouTubeWebhook(w http.ResponseWriter, r *http.Request) {
	webhook.Handler(w, r)
}
