package main

import (
	"strings"
	"testing"
)

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Direct video ID",
			input:    "VO6XEQIsCoM",
			expected: "VO6XEQIsCoM",
		},
		{
			name:     "YouTube full URL",
			input:    "https://www.youtube.com/watch?v=VO6XEQIsCoM",
			expected: "VO6XEQIsCoM",
		},
		{
			name:     "YouTube short URL",
			input:    "https://youtu.be/VO6XEQIsCoM",
			expected: "VO6XEQIsCoM",
		},
		{
			name:     "YouTube URL with additional parameters",
			input:    "https://www.youtube.com/watch?v=VO6XEQIsCoM&t=123",
			expected: "VO6XEQIsCoM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractVideoID(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractVideoID(%s) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetTranscript_ErrorCases(t *testing.T) {
	client := NewClient()

	tests := []struct {
		name    string
		videoID string
		wantErr bool
		errType interface{}
	}{
		{
			name:    "Invalid video ID",
			videoID: "invalid_id",
			wantErr: true,
			errType: &ErrVideoUnavailable{},
		},
		{
			name:    "Empty video ID",
			videoID: "",
			wantErr: true,
			errType: &ErrVideoUnavailable{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.GetTranscript(tt.videoID)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTranscript() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				switch tt.errType.(type) {
				case *ErrVideoUnavailable:
					if _, ok := err.(*ErrVideoUnavailable); !ok {
						t.Errorf("GetTranscript() error = %v, want %T", err, tt.errType)
					}
				}
			}
		})
	}
}

func TestGetTranscript_Functional(t *testing.T) {
	// Initialize client
	client := NewClient()

	// Test video URL: https://www.youtube.com/watch?v=VO6XEQIsCoM
	videoID := "VO6XEQIsCoM"

	// Test GetTranscript
	entries, err := client.GetTranscript(videoID)
	if err != nil {
		t.Fatalf("Failed to get transcript: %v", err)
	}

	// Validate transcript entries
	if len(entries) == 0 {
		t.Error("Expected non-empty transcript entries")
	}

	for i, entry := range entries {
		// Validate entry structure
		if entry.Start < 0 {
			t.Errorf("Entry %d: Invalid start time: %f", i, entry.Start)
		}
		if entry.Duration <= 0 {
			t.Errorf("Entry %d: Invalid duration: %f", i, entry.Duration)
		}
		if strings.TrimSpace(entry.Text) == "" {
			t.Errorf("Entry %d: Empty text", i)
		}
	}

	// Test GetTranscriptString
	transcriptStr, err := client.GetTranscriptString(videoID)
	if err != nil {
		t.Fatalf("Failed to get transcript string: %v", err)
	}

	if strings.TrimSpace(transcriptStr) == "" {
		t.Error("Expected non-empty transcript string")
	}

	// Validate that the string contains text from entries
	if !strings.Contains(transcriptStr, entries[0].Text) {
		t.Error("Transcript string doesn't contain expected text from first entry")
	}
}
