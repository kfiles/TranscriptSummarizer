package pipeline

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/storage"
	"github.com/kfiles/transcriptsummarizer/pkg/db"
	"github.com/kfiles/transcriptsummarizer/pkg/facebook"
	"github.com/kfiles/transcriptsummarizer/pkg/summarize"
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const defaultHugoContentDir = "site/content/minutes"
const indexName = "_index.md"

var (
	newTranscriber = transcript.NewVideoTranscriber
	doSummarize    = summarize.Summarize
	listNames      = db.ListOfficialNames
)

// Run processes a single video: stores metadata in MongoDB, extracts and summarizes the
// transcript, writes Hugo markdown, and optionally uploads to GCS and triggers Cloud Build.
//
// Env vars consulted (beyond those in sub-packages):
//
//	HUGO_CONTENT_DIR   - where to write .md files (default: site/content/minutes)
//	GCS_BUCKET         - if set, new markdown files are uploaded here after writing
//	PUBSUB_PROJECT     - GCP project ID for Pub/Sub (required when GCS_BUCKET is set)
//	PUBSUB_TOPIC       - Pub/Sub topic to publish on after upload (triggers Cloud Build)
//	FACEBOOK_PAGE_ID   - optional; enables Facebook posting
//	FACEBOOK_PAGE_TOKEN - optional; enables Facebook posting
func Run(ctx context.Context, facade db.Facade, client *mongo.Client, v *transcript.Video) error {
	hugoContentDir := os.Getenv("HUGO_CONTENT_DIR")
	if hugoContentDir == "" {
		hugoContentDir = defaultHugoContentDir
	}

	log.Printf("pipeline: checking if video %s exists in database", v.VideoId)
	_, verr := facade.GetVideo(ctx, client, v.VideoId)
	if verr != nil {
		log.Printf("pipeline: video %s not found, inserting metadata", v.VideoId)
		if err := facade.InsertVideo(ctx, client, v); err != nil {
			return fmt.Errorf("insert video %s: %w", v.VideoId, err)
		}
		log.Printf("pipeline: inserted metadata for video %s", v.VideoId)
	} else {
		log.Printf("pipeline: video %s already exists in database", v.VideoId)
	}

	log.Printf("pipeline: transcribing video %s", v.VideoId)
	text, lang, err := newTranscriber().Transcribe(ctx, v.VideoId)
	if err != nil {
		return fmt.Errorf("transcribe %s: %w", v.VideoId, err)
	}
	if text == "" {
		return fmt.Errorf("empty transcript for video %s", v.VideoId)
	}
	log.Printf("pipeline: transcribed video %s: lang=%s, length=%d chars", v.VideoId, lang, len(text))

	log.Printf("pipeline: fetching official names for video %s", v.VideoId)
	names, err := listNames(ctx, client)
	if err != nil {
		log.Printf("pipeline: fetch official names: %v — continuing without name list", err)
		names = nil
	} else {
		log.Printf("pipeline: fetched %d official names", len(names))
	}

	log.Printf("pipeline: processing transcript for video %s (lang=%s)", v.VideoId, lang)
	if err := processTranscript(ctx, facade, client, v, text, lang, names, hugoContentDir); err != nil {
		return fmt.Errorf("transcript %s for video %s: %w", lang, v.VideoId, err)
	}
	log.Printf("pipeline: completed processing for video %s", v.VideoId)
	return nil
}

func processTranscript(ctx context.Context, facade db.Facade, client *mongo.Client, v *transcript.Video, text, languageCode string, names []string, hugoContentDir string) error {
	newT := transcript.NewTranscript(v.VideoId, languageCode, text)

	log.Printf("pipeline: checking for existing transcript for video %s (lang=%s)", v.VideoId, languageCode)
	t, trerr := facade.GetTranscript(ctx, client, newT.VideoId, newT.LanguageCode)
	if trerr != nil {
		log.Printf("pipeline: no existing transcript for video %s, inserting", v.VideoId)
		if err := facade.InsertTranscript(ctx, client, newT); err != nil {
			return fmt.Errorf("insert transcript: %w", err)
		}
		log.Printf("pipeline: stored transcript for video %s (%d chars)", v.VideoId, len(text))

		log.Printf("pipeline: summarizing transcript for video %s (%d chars, %d names)", v.VideoId, len(newT.RetrievedText), len(names))
		summary, serr := doSummarize(ctx, newT.RetrievedText, names)
		if serr != nil {
			return fmt.Errorf("summarize: %w", serr)
		}
		log.Printf("pipeline: summary generated for video %s (%d chars)", v.VideoId, len(summary))
		newT.SummaryText = summary
		t = newT

		log.Printf("pipeline: updating transcript record for video %s with summary", v.VideoId)
		if err := facade.UpdateTranscript(ctx, client, newT); err != nil {
			log.Printf("pipeline: update transcript for %s: %v", v.VideoId, err)
		}
	} else {
		log.Printf("pipeline: found existing transcript for video %s (lang=%s)", v.VideoId, languageCode)
	}

	log.Printf("pipeline: writing markdown for video %s to %s", v.VideoId, hugoContentDir)
	mdPath, err := writeMarkdown(v, t, hugoContentDir)
	if err != nil {
		return fmt.Errorf("write markdown: %w", err)
	}
	log.Printf("pipeline: markdown written to %s", mdPath)

	if bucket := os.Getenv("GCS_BUCKET"); bucket != "" {
		log.Printf("pipeline: uploading %s to GCS bucket %s", mdPath, bucket)
		if err := uploadToGCS(ctx, bucket, hugoContentDir, mdPath); err != nil {
			log.Printf("pipeline: GCS upload for %s: %v", v.VideoId, err)
		} else {
			log.Printf("pipeline: GCS upload complete for video %s", v.VideoId)
			log.Printf("pipeline: publishing build trigger for video %s", v.VideoId)
			if err := publishBuildTrigger(ctx, v.VideoId); err != nil {
				log.Printf("pipeline: Pub/Sub publish for %s: %v", v.VideoId, err)
			} else {
				log.Printf("pipeline: build trigger published for video %s", v.VideoId)
			}
		}
	}

	fbPageID := os.Getenv("FACEBOOK_PAGE_ID")
	fbToken := os.Getenv("FACEBOOK_PAGE_TOKEN")
	if os.Getenv("FACEBOOK_ENABLED") != "false" && fbPageID != "" && fbToken != "" {
		log.Printf("pipeline: posting to Facebook for video %s", v.VideoId)
		post := facebook.FormatPost(v.Title, t.SummaryText)
		if err := facebook.PostToPage(fbPageID, fbToken, post); err != nil {
			log.Printf("pipeline: Facebook post for %s: %v", v.VideoId, err)
		} else {
			log.Printf("pipeline: Facebook post complete for video %s", v.VideoId)
		}
	}
	return nil
}

func writeMarkdown(v *transcript.Video, t *transcript.Transcript, hugoContentDir string) (string, error) {
	dirPath := path.Join(hugoContentDir, v.PublishedAt.Format("2006"), v.PublishedAt.Format("January"))
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", err
	}
	createIndex(path.Join(hugoContentDir, v.PublishedAt.Format("2006")))
	createIndex(dirPath)

	markdown := fmt.Sprintf("+++\ntitle = '%s'\ndate = %s\ndraft = false\n+++\n",
		v.Title, v.PublishedAt.Format("2006-01-02T15:04:05-07:00"))
	markdown += t.SummaryText

	mdPath := path.Join(dirPath, v.VideoId+".md")
	return mdPath, os.WriteFile(mdPath, []byte(markdown), 0644)
}

func createIndex(dirPath string) {
	f, err := os.OpenFile(path.Join(dirPath, indexName), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Printf("create index in %s: %v", dirPath, err)
		return
	}
	f.Close()
}

// uploadToGCS uploads a single markdown file to the GCS bucket, preserving the
// relative path under hugoContentDir as the object name.
func uploadToGCS(ctx context.Context, bucket, hugoContentDir, mdPath string) error {
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("storage client: %w", err)
	}
	defer gcsClient.Close()

	relPath, err := filepath.Rel(hugoContentDir, mdPath)
	if err != nil {
		return fmt.Errorf("rel path: %w", err)
	}
	objectName := "minutes/" + strings.ReplaceAll(relPath, string(filepath.Separator), "/")

	f, err := os.Open(mdPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	wc := gcsClient.Bucket(bucket).Object(objectName).NewWriter(ctx)
	if _, err = io.Copy(wc, f); err != nil {
		wc.Close()
		return fmt.Errorf("upload: %w", err)
	}
	return wc.Close()
}

// publishBuildTrigger publishes a Pub/Sub message that Cloud Build listens to.
func publishBuildTrigger(ctx context.Context, videoID string) error {
	project := os.Getenv("PUBSUB_PROJECT")
	topic := os.Getenv("PUBSUB_TOPIC")
	if project == "" || topic == "" {
		return fmt.Errorf("PUBSUB_PROJECT and PUBSUB_TOPIC must be set")
	}

	psClient, err := pubsub.NewClient(ctx, project)
	if err != nil {
		return fmt.Errorf("pubsub client: %w", err)
	}
	defer psClient.Close()

	publisher := psClient.Publisher(topic)
	result := publisher.Publish(ctx, &pubsub.Message{
		Data: []byte(fmt.Sprintf(`{"videoId":%q}`, videoID)),
	})
	_, err = result.Get(ctx)
	return err
}
