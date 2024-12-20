package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	ytw "github.com/mjlefevre/yt-words-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <YouTube URL or Video ID>\n", getBinaryName())
		os.Exit(1)
	}

	input := os.Args[1]
	videoID := extractVideoID(input)

	client := ytw.NewClient()

	transcript, err := client.GetTranscriptString(videoID)
	if err != nil {
		log.Fatalf("Error fetching transcript: %v", err)
	}

	fmt.Printf("Transcript for video %s:\n%s\n", videoID, transcript)
}

func getBinaryName() string {
	return "yt-words"
}

func extractVideoID(input string) string {
	// Check if the input is already a video ID
	if len(input) == 11 && !strings.Contains(input, "/") && !strings.Contains(input, ".") {
		return input
	}

	// Handle youtube.com URLs
	if strings.Contains(input, "youtube.com/watch") {
		if u, err := url.Parse(input); err == nil {
			if values, err := url.ParseQuery(u.RawQuery); err == nil {
				if videoID := values.Get("v"); videoID != "" {
					return videoID
				}
			}
		}
	}

	// Handle youtu.be URLs
	if strings.Contains(input, "youtu.be/") {
		if parts := strings.Split(input, "youtu.be/"); len(parts) == 2 {
			return strings.Split(parts[1], "?")[0]
		}
	}

	log.Fatalf("Invalid YouTube URL or Video ID: %s", input)
	return ""
}
