package pipeline

import (
	"context"
	"errors"
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

	_, verr := facade.GetVideo(ctx, client, v.VideoId)
	videoExists := verr == nil
	if videoExists {
		log.Printf("pipeline: video %s already exists in database", v.VideoId)
	}

	log.Printf("pipeline: transcribing video %s", v.VideoId)
	text, lang, err := newTranscriber().Transcribe(ctx, v.VideoId)
	if err != nil {
		if errors.Is(err, transcript.ErrTranscriptUnavailable) {
			log.Printf("pipeline: transcript unavailable for video %s — recording and skipping Facebook post", v.VideoId)
			if !videoExists {
				v.Description = "Transcript unavailable"
				if insErr := facade.InsertVideo(ctx, client, v); insErr != nil {
					return fmt.Errorf("insert video %s: %w", v.VideoId, insErr)
				}
			}
			return nil
		}
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

	if !videoExists {
		log.Printf("pipeline: inserting metadata for video %s", v.VideoId)
		if err := facade.InsertVideo(ctx, client, v); err != nil {
			return fmt.Errorf("insert video %s: %w", v.VideoId, err)
		}
		log.Printf("pipeline: inserted metadata for video %s", v.VideoId)
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
		var transcriptURL string
		if projectID := os.Getenv("PROJECT_ID"); projectID != "" {
			transcriptURL = facebook.TranscriptURL(projectID, v.VideoId, v.PublishedAt)
		}
		post := facebook.FormatPost(v.Title, t.SummaryText, transcriptURL)
		if err := facebook.PostToPage(fbPageID, fbToken, post); err != nil {
			log.Printf("pipeline: Facebook post for %s: %v", v.VideoId, err)
		} else {
			log.Printf("pipeline: Facebook post complete for video %s", v.VideoId)
		}
	}
	return nil
}

// WriteAllMarkdown writes Hugo markdown for every video in the collection that
// has valid metadata and a non-empty summary. It is called after new videos are
// processed so that a from-scratch Hugo build always includes the full catalogue.
func WriteAllMarkdown(ctx context.Context, facade db.Facade, client *mongo.Client) error {
	hugoContentDir := os.Getenv("HUGO_CONTENT_DIR")
	if hugoContentDir == "" {
		hugoContentDir = defaultHugoContentDir
	}

	videos, err := facade.ListAllVideos(ctx, client)
	if err != nil {
		return fmt.Errorf("list all videos: %w", err)
	}

	var written []string
	for _, v := range videos {
		if v.VideoId == "" || v.Title == "" || v.PublishedAt.IsZero() {
			continue
		}
		transcripts, err := facade.ListTranscripts(ctx, client, v.VideoId)
		if err != nil {
			log.Printf("pipeline: list transcripts for %s: %v", v.VideoId, err)
			continue
		}
		var t *transcript.Transcript
		for _, tr := range transcripts {
			if tr.SummaryText != "" {
				t = tr
				break
			}
		}
		if t == nil {
			continue
		}
		mdPath, err := writeMarkdown(v, t, hugoContentDir)
		if err != nil {
			log.Printf("pipeline: write markdown for %s: %v", v.VideoId, err)
			continue
		}
		written = append(written, mdPath)
	}
	log.Printf("pipeline: wrote markdown for %d/%d videos", len(written), len(videos))

	if bucket := os.Getenv("GCS_BUCKET"); bucket != "" && len(written) > 0 {
		for _, mdPath := range written {
			if err := uploadToGCS(ctx, bucket, hugoContentDir, mdPath); err != nil {
				log.Printf("pipeline: GCS upload %s: %v", mdPath, err)
			}
		}
		if err := publishBuildTrigger(ctx, "all"); err != nil {
			log.Printf("pipeline: build trigger: %v", err)
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
