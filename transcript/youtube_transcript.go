package transcript

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// Error types
type ErrVideoUnavailable struct {
	VideoID string
}

func (e ErrVideoUnavailable) Error() string {
	return fmt.Sprintf("Video %s is unavailable", e.VideoID)
}

type ErrNoTranscriptFound struct {
	VideoID string
}

func (e ErrNoTranscriptFound) Error() string {
	return fmt.Sprintf("No transcript found for video %s", e.VideoID)
}

type ErrTranscriptsDisabled struct {
	VideoID string
}

func (e ErrTranscriptsDisabled) Error() string {
	return fmt.Sprintf("Transcripts are disabled for video %s", e.VideoID)
}

// Client represents the YouTube Transcript API client
type Client struct {
	httpClient *http.Client
}

// Transcript represents a single transcript
type Transcript struct {
	BaseURL      string
	LanguageCode string
	Language     string
	IsGenerated  bool
}

// TranscriptEntry represents a single entry in the transcript
type TranscriptEntry struct {
	Text     string
	Start    float64
	Duration float64
}

// NewClient creates a new YouTube Transcript API client
func NewClient(options ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{},
	}
	for _, opt := range options {
		opt(c)
	}
	return c
}

// ClientOption defines a function to configure the Client
type ClientOption func(*Client)

// WithProxy sets a proxy for the HTTP client
func WithProxy(proxyURLStr string) ClientOption {
	return func(c *Client) {
		parsedURL, err := url.Parse(proxyURLStr)
		if err != nil {
			log.Printf("Error parsing proxy URL: %v", err)
			return
		}
		c.httpClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(parsedURL),
		}
	}
}

// GetTranscript fetches the transcript for a given video ID, preferring English if available
func (c *Client) GetTranscript(videoID string) ([]TranscriptEntry, error) {
	videoInfo, err := c.fetchVideoInfo(videoID)
	if err != nil {
		return nil, err
	}

	transcripts, err := extractTranscriptData(videoInfo)
	if err != nil {
		return nil, err
	}

	if len(transcripts) == 0 {
		return nil, ErrNoTranscriptFound{VideoID: videoID}
	}

	// Try to find English transcript first
	var selectedTranscript Transcript
	for _, t := range transcripts {
		if strings.HasPrefix(t.LanguageCode, "en") { // Matches 'en', 'en-US', 'en-GB', etc.
			selectedTranscript = t
			break
		}
	}

	// If no English transcript found, fall back to the first available one
	if selectedTranscript.BaseURL == "" {
		selectedTranscript = transcripts[0]
	}

	return c.fetchTranscript(selectedTranscript)
}

// GetTranscriptString fetches the transcript and returns it as a single string
func (c *Client) GetTranscriptString(videoID string) (string, error) {
	entries, err := c.GetTranscript(videoID)
	if err != nil {
		return "", err
	}
	return ConcatenateTranscript(entries), nil
}

// ConcatenateTranscript combines all transcript entries into a single string
func ConcatenateTranscript(entries []TranscriptEntry) string {
	var builder strings.Builder
	for i, entry := range entries {
		builder.WriteString(entry.Text)
		if i < len(entries)-1 {
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func (c *Client) fetchVideoInfo(videoID string) (string, error) {
	if strings.TrimSpace(videoID) == "" {
		return "", &ErrVideoUnavailable{VideoID: videoID}
	}

	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	resp, err := c.httpClient.Get(videoURL)
	if err != nil {
		return "", &ErrVideoUnavailable{VideoID: videoID}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &ErrVideoUnavailable{VideoID: videoID}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func extractTranscriptData(videoInfo string) ([]Transcript, error) {
	startMarker := "\"captions\":"
	startIndex := strings.Index(videoInfo, startMarker)
	if startIndex == -1 {
		// If we can't find captions data, the video is likely unavailable
		return nil, &ErrVideoUnavailable{VideoID: ""}
	}

	// Find the opening brace of the JSON object
	jsonStart := strings.Index(videoInfo[startIndex:], "{")
	if jsonStart == -1 {
		return nil, fmt.Errorf("could not find the start of JSON object")
	}
	jsonStart += startIndex

	// Find the closing brace of the JSON object
	braceCount := 1
	jsonEnd := -1
	for i := jsonStart + 1; i < len(videoInfo); i++ {
		if videoInfo[i] == '{' {
			braceCount++
		} else if videoInfo[i] == '}' {
			braceCount--
			if braceCount == 0 {
				jsonEnd = i + 1
				break
			}
		}
	}

	if jsonEnd == -1 {
		return nil, fmt.Errorf("could not find the end of JSON object")
	}

	captionsJSON := videoInfo[jsonStart:jsonEnd]

	// Check if the extracted JSON is empty or too short
	if len(captionsJSON) < 10 {
		return nil, fmt.Errorf("extracted JSON is too short or empty: %s", captionsJSON)
	}

	var transcriptData map[string]interface{}
	err := json.Unmarshal([]byte(captionsJSON), &transcriptData)
	if err != nil {
		return nil, fmt.Errorf("error parsing captions JSON: %v\nJSON: %s", err, captionsJSON)
	}

	playerCaptionsTracklistRenderer, ok := transcriptData["playerCaptionsTracklistRenderer"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("playerCaptionsTracklistRenderer not found in JSON")
	}

	captionTracks, ok := playerCaptionsTracklistRenderer["captionTracks"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("captionTracks not found in playerCaptionsTracklistRenderer")
	}

	var transcripts []Transcript
	for _, track := range captionTracks {
		trackMap, ok := track.(map[string]interface{})
		if !ok {
			continue
		}

		baseURL, _ := trackMap["baseUrl"].(string)
		languageCode, _ := trackMap["languageCode"].(string)
		name, _ := trackMap["name"].(map[string]interface{})
		simpleText, _ := name["simpleText"].(string)
		kind, _ := trackMap["kind"].(string)

		transcripts = append(transcripts, Transcript{
			BaseURL:      baseURL,
			LanguageCode: languageCode,
			Language:     simpleText,
			IsGenerated:  kind == "asr",
		})
	}

	return transcripts, nil
}

func (c *Client) fetchTranscript(transcript Transcript) ([]TranscriptEntry, error) {
	resp, err := c.httpClient.Get(transcript.BaseURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var transcriptResp struct {
		XMLName xml.Name `xml:"transcript"`
		Texts   []struct {
			Start float64 `xml:"start,attr"`
			Dur   float64 `xml:"dur,attr"`
			Text  string  `xml:",chardata"`
		} `xml:"text"`
	}

	if err := xml.NewDecoder(resp.Body).Decode(&transcriptResp); err != nil {
		return nil, err
	}

	var entries []TranscriptEntry
	for _, text := range transcriptResp.Texts {
		entries = append(entries, TranscriptEntry{
			Text:     html.UnescapeString(text.Text), // Decode HTML entities
			Start:    text.Start,
			Duration: text.Dur,
		})
	}

	return entries, nil
}

// GetTranscriptWithLanguage fetches the transcript for a given video ID in the specified language code
// If the specified language is not available, it returns an error
func (c *Client) GetTranscriptWithLanguage(videoID string, languageCode string) ([]TranscriptEntry, error) {
	videoInfo, err := c.fetchVideoInfo(videoID)
	if err != nil {
		return nil, err
	}

	transcripts, err := extractTranscriptData(videoInfo)
	if err != nil {
		return nil, err
	}

	if len(transcripts) == 0 {
		return nil, ErrNoTranscriptFound{VideoID: videoID}
	}

	// Try to find transcript in specified language
	for _, t := range transcripts {
		if strings.HasPrefix(t.LanguageCode, languageCode) {
			return c.fetchTranscript(t)
		}
	}

	return nil, fmt.Errorf("no transcript found for language code: %s", languageCode)
}

// ListAvailableTranscripts returns a list of available transcript languages for a video
func (c *Client) ListAvailableTranscripts(videoID string) ([]Transcript, error) {
	videoInfo, err := c.fetchVideoInfo(videoID)
	if err != nil {
		return nil, err
	}

	return extractTranscriptData(videoInfo)
}

// FetchMultipleTranscripts fetches transcripts for multiple video IDs concurrently
func (c *Client) FetchMultipleTranscripts(videoIDs []string) map[string][]TranscriptEntry {
	results := make(map[string][]TranscriptEntry)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, id := range videoIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			transcript, err := c.GetTranscript(id)
			if err == nil {
				mu.Lock()
				results[id] = transcript
				mu.Unlock()
			}
		}(id)
	}

	wg.Wait()
	return results
}

// ExtractVideoID extracts the video ID from various YouTube URL formats or returns the ID directly.
// It supports full youtube.com URLs, short youtu.be URLs, and direct video IDs.
func ExtractVideoID(input string) string {
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

	return ""
}
