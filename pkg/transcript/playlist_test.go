package transcript

import (
	"errors"
	"os"
	"testing"
	"time"

	"google.golang.org/api/youtube/v3"
)

func fakeItem(videoID, playlistID string, position int64, publishedAt string) *youtube.PlaylistItem {
	return &youtube.PlaylistItem{
		Snippet: &youtube.PlaylistItemSnippet{
			PlaylistId:  playlistID,
			PublishedAt: publishedAt,
			Position:    position,
			Title:       "Title:" + videoID,
			Description: "Desc:" + videoID,
			ResourceId:  &youtube.ResourceId{VideoId: videoID},
		},
	}
}

func setFakeFetcher(t *testing.T, fn func(string) fetchPage) {
	t.Helper()
	orig := newPlaylistFetcher
	newPlaylistFetcher = fn
	t.Cleanup(func() { newPlaylistFetcher = orig })
}

func TestScanPlaylist_MissingAPIKey(t *testing.T) {
	os.Unsetenv("YOUTUBE_API_KEY")
	_, err := ScanPlaylist("pl1", "", 50)
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
}

func TestScanPlaylist_FetchError(t *testing.T) {
	t.Setenv("YOUTUBE_API_KEY", "fake-key")
	setFakeFetcher(t, func(_ string) fetchPage {
		return func(_, _ string, _ int64) ([]*youtube.PlaylistItem, string, error) {
			return nil, "", errors.New("api error")
		}
	})

	_, err := ScanPlaylist("pl1", "", 50)
	if err == nil {
		t.Fatal("expected error from fetcher, got nil")
	}
}

func TestScanPlaylist_SinglePage(t *testing.T) {
	t.Setenv("YOUTUBE_API_KEY", "fake-key")
	setFakeFetcher(t, func(_ string) fetchPage {
		return func(_, _ string, _ int64) ([]*youtube.PlaylistItem, string, error) {
			return []*youtube.PlaylistItem{
				fakeItem("v1", "pl1", 0, "2024-01-01T00:00:00Z"),
				fakeItem("v2", "pl1", 1, "2024-01-02T00:00:00Z"),
			}, "", nil
		}
	})

	entries, err := ScanPlaylist("pl1", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].Video.VideoId != "v1" || entries[1].Video.VideoId != "v2" {
		t.Errorf("unexpected order: %s, %s", entries[0].Video.VideoId, entries[1].Video.VideoId)
	}
	if entries[0].PageToken != "" {
		t.Errorf("pageToken = %q, want empty (first page started from empty token)", entries[0].PageToken)
	}
}

func TestScanPlaylist_SortsByPublishedAt(t *testing.T) {
	t.Setenv("YOUTUBE_API_KEY", "fake-key")
	setFakeFetcher(t, func(_ string) fetchPage {
		return func(_, _ string, _ int64) ([]*youtube.PlaylistItem, string, error) {
			// Return items in reverse chronological order.
			return []*youtube.PlaylistItem{
				fakeItem("v3", "pl1", 2, "2024-01-03T00:00:00Z"),
				fakeItem("v1", "pl1", 0, "2024-01-01T00:00:00Z"),
				fakeItem("v2", "pl1", 1, "2024-01-02T00:00:00Z"),
			}, "", nil
		}
	})

	entries, err := ScanPlaylist("pl1", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"v1", "v2", "v3"}
	for i, e := range entries {
		if e.Video.VideoId != want[i] {
			t.Errorf("entries[%d].VideoId = %q, want %q", i, e.Video.VideoId, want[i])
		}
	}
}

func TestScanPlaylist_Pagination(t *testing.T) {
	t.Setenv("YOUTUBE_API_KEY", "fake-key")
	setFakeFetcher(t, func(_ string) fetchPage {
		return func(_, pageToken string, _ int64) ([]*youtube.PlaylistItem, string, error) {
			if pageToken == "" {
				return []*youtube.PlaylistItem{
					fakeItem("v1", "pl1", 0, "2024-01-01T00:00:00Z"),
				}, "page2", nil
			}
			return []*youtube.PlaylistItem{
				fakeItem("v2", "pl1", 1, "2024-01-02T00:00:00Z"),
			}, "", nil
		}
	})

	entries, err := ScanPlaylist("pl1", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].PageToken != "" {
		t.Errorf("page1 entry pageToken = %q, want empty", entries[0].PageToken)
	}
	if entries[1].PageToken != "page2" {
		t.Errorf("page2 entry pageToken = %q, want %q", entries[1].PageToken, "page2")
	}
}

func TestScanPlaylist_StartPageToken(t *testing.T) {
	t.Setenv("YOUTUBE_API_KEY", "fake-key")

	var gotToken string
	setFakeFetcher(t, func(_ string) fetchPage {
		return func(_, pageToken string, _ int64) ([]*youtube.PlaylistItem, string, error) {
			gotToken = pageToken
			return nil, "", nil
		}
	})

	if _, err := ScanPlaylist("pl1", "start-token", 50); err != nil {
		t.Fatal(err)
	}
	if gotToken != "start-token" {
		t.Errorf("first page token = %q, want %q", gotToken, "start-token")
	}
}

func TestScanPlaylist_PageSizePassedToFetcher(t *testing.T) {
	t.Setenv("YOUTUBE_API_KEY", "fake-key")

	var gotSize int64
	setFakeFetcher(t, func(_ string) fetchPage {
		return func(_, _ string, pageSize int64) ([]*youtube.PlaylistItem, string, error) {
			gotSize = pageSize
			return nil, "", nil
		}
	})

	if _, err := ScanPlaylist("pl1", "", 25); err != nil {
		t.Fatal(err)
	}
	if gotSize != 25 {
		t.Errorf("pageSize = %d, want 25", gotSize)
	}
}

func TestScanPlaylist_UnparsableDateDefaultsToZero(t *testing.T) {
	t.Setenv("YOUTUBE_API_KEY", "fake-key")
	setFakeFetcher(t, func(_ string) fetchPage {
		return func(_, _ string, _ int64) ([]*youtube.PlaylistItem, string, error) {
			return []*youtube.PlaylistItem{
				fakeItem("v1", "pl1", 0, "not-a-date"),
			}, "", nil
		}
	})

	entries, err := ScanPlaylist("pl1", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1", len(entries))
	}
	if !entries[0].Video.PublishedAt.Equal(time.Time{}) {
		t.Errorf("PublishedAt = %v, want zero value", entries[0].Video.PublishedAt)
	}
}
