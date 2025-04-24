package transcript

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const baseURL = "https://www.youtube.com/watch?v="

type ytInitialPlayerResponse struct {
	Captions struct {
		PlayerCaptionsTracklistRenderer struct {
			CaptionTracks []struct {
				BaseUrl      string `json:"baseUrl"`
				LanguageCode string `json:"languageCode"`
			} `json:"captionTracks"`
		} `json:"playerCaptionsTracklistRenderer"`
	} `json:"captions"`
}

type Caption struct {
	BaseUrl      string
	LanguageCode string
}

type Transcript struct {
	Id            string `bson:"_id" json:"id"`
	VideoId       string `bson:"videoId" json:"videoId"`
	LanguageCode  string `bson:"languageCode" json:"languageCode"`
	RetrievedText string `bson:"retrievedText" json:"retrievedText"`
	SummaryText   string `bson:"summaryText" json:"summaryText"`
}

func (t *Transcript) SetId() error {
	if t.VideoId == "" || t.LanguageCode == "" {
		return fmt.Errorf("Transcript id or language code not set")
	}
	t.Id = t.VideoId + "_" + t.LanguageCode
	return nil
}

func NewTranscript(videoId string, languageCode string, retrievedText string) *Transcript {
	t := Transcript{
		VideoId:       videoId,
		LanguageCode:  languageCode,
		RetrievedText: retrievedText,
	}
	t.SetId()
	return &t
}

type CaptionResponse struct {
	Text []string `xml:"text"`
}

// decodeDoubleEncodedString recursively decodes XML entities in a string.
func decodeDoubleEncodedString(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&#34;", "\"")
	return s
}

func (c *Caption) Download() (*CaptionResponse, error) {
	resp, err := http.Get(c.BaseUrl)
	if err != nil {
		return nil, fmt.Errorf("unable to download caption: %w", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Error reading body:", err)
	}

	var t CaptionResponse
	err = xml.Unmarshal([]byte(body), &t)
	for i, text := range t.Text {
		t.Text[i] = decodeDoubleEncodedString(text)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to parse xml: %w", err)
	}

	return &t, nil
}

func (c *Caption) ExtractText() (string, error) {
	transcript, err := c.Download()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, t := range transcript.Text {
		sb.WriteString(t)
		sb.WriteString(" ")
	}
	return sb.String(), nil
}

func ListVideoCaptions(videoID string) ([]Caption, error) {
	resp, err := http.Get(baseURL + videoID)
	if err != nil {
		return nil, fmt.Errorf("unable to download video page: %w", err)
	}

	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body: %w", err)
	}

	pageContent := string(content)

	// Find ytInitialPlayerResponse variable
	pageContentSplited := strings.Split(pageContent, "ytInitialPlayerResponse = ")
	if len(pageContentSplited) < 2 {
		return nil, fmt.Errorf("unable to find ytInitialPlayerResponse variable")
	}

	// Find the end of the variable
	pageContentSplited = strings.Split(pageContentSplited[1], ";</script>")
	if len(pageContentSplited) < 2 {
		return nil, fmt.Errorf("unable to find the end of the ytInitialPlayerResponse variable")
	}

	ytInitialPlayerResponse := ytInitialPlayerResponse{}
	err = json.Unmarshal([]byte(pageContentSplited[0]), &ytInitialPlayerResponse)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal ytInitialPlayerResponse: %w", err)
	}

	captions := make([]Caption, 0, len(ytInitialPlayerResponse.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks))
	for _, caption := range ytInitialPlayerResponse.Captions.PlayerCaptionsTracklistRenderer.CaptionTracks {
		captions = append(captions, Caption{
			BaseUrl:      caption.BaseUrl,
			LanguageCode: caption.LanguageCode,
		})
	}

	return captions, nil
}
