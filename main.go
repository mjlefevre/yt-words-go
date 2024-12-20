package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mjlefevre/yt-words-go/transcript"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <YouTube URL or Video ID>\n", getBinaryName())
		os.Exit(1)
	}

	input := os.Args[1]
	videoID := transcript.ExtractVideoID(input)
	if videoID == "" {
		log.Fatalf("Invalid YouTube URL or Video ID: %s", input)
	}

	client := transcript.NewClient()
	transcriptText, err := client.GetTranscriptString(videoID)
	if err != nil {
		log.Fatalf("Error fetching transcript: %v", err)
	}

	fmt.Printf("Transcript for video %s:\n%s\n", videoID, transcriptText)
}

func getBinaryName() string {
	return "yt-words"
}
