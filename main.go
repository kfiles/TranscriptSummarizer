package main

import (
	"fmt"
	"log"

	"github.com/kfiles/transcriptsummarizer/pkg/db"
	"github.com/kfiles/transcriptsummarizer/pkg/pipeline"
	"github.com/kfiles/transcriptsummarizer/pkg/transcript"
	"golang.org/x/net/context"
)

const playlistId = "PLk5NCS3UGBusb9TkuXtVQhrOhRyezF63S"

func main() {
	videos, err := transcript.GetPlaylistItems(playlistId)
	if err != nil {
		log.Fatal(err)
	}
	facade := db.NewFacade()
	client, dberr := db.NewClient()
	if dberr != nil {
		log.Fatalf("Unable to connect to database %v", dberr)
	}
	defer func() {
		if err := client.Disconnect(context.TODO()); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()

	for _, v := range videos {
		fmt.Println(v.VideoId, ": ", v.Title, " - ", v.PublishedAt)
		if err := pipeline.Run(context.Background(), facade, client, v); err != nil {
			log.Printf("pipeline error for video %s: %v", v.VideoId, err)
		}
	}
}
