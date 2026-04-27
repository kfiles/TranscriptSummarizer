package main

import (
	"fmt"
	"github.com/kfiles/transcriptsummarizer/pkg/db"
	"github.com/kfiles/transcriptsummarizer/pkg/summarize"
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
	"golang.org/x/net/context"
	"log"
	"os"
	"path"
)

const channelId = "UCGnv43oWpciURP-bTDc3GnA"

const playlistId = "PLk5NCS3UGBusb9TkuXtVQhrOhRyezF63S"
const indexName = "_index.md"
const hugoSiteDir = "site/content/minutes"

func createHugoIndex(dirPath string) error {
	file, err := os.OpenFile(path.Join(dirPath, indexName), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	return nil
}

func main() {
	videos, err := transcript.GetPlaylistItems(playlistId)
	if err != nil {
		log.Fatal(err)
	}
	facade := db.NewFacade()
	client, dberr := db.NewClient()
	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()
	if dberr != nil {
		log.Fatalf("Unable to connect to database %v", dberr)
	}
	for _, v := range videos {

		// Print the playlist ID and title for the playlist resource.
		fmt.Println(v.VideoId, ": ", v.Title, " - ", v.PublishedAt)
		_, verr := facade.GetVideo(context.Background(), client, v.VideoId)
		if verr != nil {
			err = facade.InsertVideo(context.Background(), client, v)
			if err != nil {
				log.Fatalf("Unable to insert video %v", err)
			}
		}
		captions, terr := transcript.ListVideoCaptions(v.VideoId)
		if terr != nil {
			log.Printf("Error fetching caption files for video %s", v.VideoId)
			continue
		}
		for _, c := range captions {
			text, cerr := c.ExtractText()
			if cerr != nil {
				log.Printf("Error downloading transcript for video %s", v.VideoId)
				continue
			}
			newT := transcript.NewTranscript(v.VideoId, c.LanguageCode, text)
			t, trerr := facade.GetTranscript(context.Background(), client, newT.VideoId, newT.LanguageCode)
			if trerr != nil {
				err = facade.InsertTranscript(context.Background(), client, newT)
				if err != nil {
					log.Printf("Error inserting transcript for video %s", v.VideoId)
					continue
				}
				summary, serr := summarize.Summarize(context.Background(), newT.RetrievedText)
				if serr != nil {
					log.Printf("Error summarizing transcript for video %s: %v", v.VideoId, serr)
					continue
				}
				newT.SummaryText = summary
				t = newT
				err = facade.UpdateTranscript(context.Background(), client, newT)
				if err != nil {
					log.Printf("Error updating transcript for video %s: %v", v.VideoId, err)
				}
			}
			dirPath := path.Join(hugoSiteDir, v.PublishedAt.Format("2006"), v.PublishedAt.Format("January"))
			err = os.MkdirAll(dirPath, 0755)
			if err != nil {
				log.Printf("Error creating dir for video %s: %v", v.VideoId, err)
				continue
			}
			err = createHugoIndex(path.Join(hugoSiteDir, v.PublishedAt.Format("2006")))
			if err != nil {
				log.Printf("Error creating index for video %s: %v", v.VideoId, err)
			}
			err = createHugoIndex(path.Join(hugoSiteDir, v.PublishedAt.Format("2006"), v.PublishedAt.Format("January")))
			if err != nil {
				log.Printf("Error creating index for video %s: %v", v.VideoId, err)
			}
			markdown := fmt.Sprintf("+++\ntitle = '%s'\ndate = %s\ndraft = false\n+++\n",
				v.Title, v.PublishedAt.Format("2006-01-02T15:04:05-07:00"))
			markdown += t.SummaryText
			err = os.WriteFile(path.Join(dirPath, v.VideoId+".md"), []byte(markdown), 0644)
			if err != nil {
				log.Printf("Error writing summary for video %s: %v", v.VideoId, err)
				continue
			}
		}
	}
}
