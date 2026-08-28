package wxchannelsadapter

import (
	"encoding/json"
	"testing"

	"wx_channel/internal/database/model"
)

func TestBuildDownloadTaskPreservesNativeProfileEngagement(t *testing.T) {
	adapter := NewChannelsAdapter()
	contentJSON := json.RawMessage(`{
		"id": "123456789",
		"objectNonceId": "987654321",
		"createtime": 1720000000,
		"likeCount": 1234,
		"commentCount": 56,
		"forwardCount": 78,
		"favCount": 90,
		"contact": {
			"username": "finder_test",
			"nickname": "Test Account"
		},
		"objectDesc": {
			"description": "profile feed",
			"mediaType": 4,
			"media": [{
				"url": "https://example.com/video.mp4?encfilekey=media-key&token=media-token&foo=ignored",
				"thumbUrl": "https://example.com/cover.jpg",
				"mediaType": 4,
				"width": 1080,
				"height": 1920,
				"fileSize": 1024,
				"decodeKey": "123"
			}]
		}
	}`)

	result, err := adapter.BuildDownloadTask(contentJSON, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("build download task: %v", err)
	}
	if result == nil || result.Content == nil {
		t.Fatal("build download task returned nil content")
	}
	content := result.Content
	if content.LikeCount != 1234 {
		t.Fatalf("like count: got %d want 1234", content.LikeCount)
	}
	if content.CommentCount != 56 {
		t.Fatalf("comment count: got %d want 56", content.CommentCount)
	}
	if content.ShareCount != 78 {
		t.Fatalf("share count: got %d want 78", content.ShareCount)
	}
	if content.CollectCount != 90 {
		t.Fatalf("collect count: got %d want 90", content.CollectCount)
	}

	observed := observedEngagementFields(t, content.Metadata)
	for _, field := range []string{"like", "comment", "share", "collect"} {
		if !observed[field] {
			t.Fatalf("expected %q to be marked observed: %+v", field, observed)
		}
	}
}

func TestBuildDownloadTaskDistinguishesZeroFromMissingEngagement(t *testing.T) {
	adapter := NewChannelsAdapter()
	contentJSON := json.RawMessage(`{
		"id": "123456790",
		"objectNonceId": "987654322",
		"createtime": 1720000001,
		"likeCount": 0,
		"contact": {
			"username": "finder_test",
			"nickname": "Test Account"
		},
		"objectDesc": {
			"description": "zero like profile feed",
			"mediaType": 4,
			"media": [{
				"url": "https://example.com/video-zero.mp4?encfilekey=media-key&token=media-token",
				"mediaType": 4,
				"width": 1080,
				"height": 1920,
				"fileSize": 2048,
				"decodeKey": "456"
			}]
		}
	}`)

	result, err := adapter.BuildDownloadTask(contentJSON, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("build download task: %v", err)
	}
	if result == nil || result.Content == nil {
		t.Fatal("build download task returned nil content")
	}
	content := result.Content
	if content.LikeCount != 0 {
		t.Fatalf("explicit zero like count changed: got %d", content.LikeCount)
	}

	observed := observedEngagementFields(t, content.Metadata)
	if !observed["like"] {
		t.Fatalf("explicit zero like count must be observed: %+v", observed)
	}
	for _, field := range []string{"comment", "share", "collect", "view"} {
		if observed[field] {
			t.Fatalf("missing %q metric must remain unknown: %+v", field, observed)
		}
	}
}

func observedEngagementFields(t *testing.T, raw string) map[string]bool {
	t.Helper()
	var metadata map[string]any
	if err := json.Unmarshal([]byte(raw), &metadata); err != nil {
		t.Fatalf("decode content metadata: %v", err)
	}
	values, ok := metadata[model.ContentMetadataEngagementObservedKey].([]any)
	if !ok {
		t.Fatalf("engagement observation metadata missing: %+v", metadata)
	}
	observed := make(map[string]bool, len(values))
	for _, value := range values {
		field, ok := value.(string)
		if ok {
			observed[field] = true
		}
	}
	return observed
}
