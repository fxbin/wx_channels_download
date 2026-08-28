package wxchannelsadapter

import (
	"encoding/json"
	"testing"

	"wx_channel/internal/database/model"
)

func TestParseEngagementCountString(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"0", 0},
		{"128", 128},
		{"1,234", 1234},
		{"1.2万", 12000},
		{"3.5w", 35000},
		{"12k", 12000},
		{"1.2m", 1200000},
		{"2亿", 200000000},
	}
	for _, test := range tests {
		got, ok := parse_engagement_count_string(test.input)
		if !ok {
			t.Fatalf("parse %q: expected success", test.input)
		}
		if got != test.want {
			t.Fatalf("parse %q: got %d want %d", test.input, got, test.want)
		}
	}
}

func TestEngagementMetricsFromNativeProfileFeed(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "feed-1",
		"likeCount": 1234,
		"commentCount": 56,
		"forwardCount": 78,
		"favCount": 90
	}`)
	metrics := engagement_metrics_from_fetch(raw)
	if !metrics.Like.Present || metrics.Like.Value != 1234 {
		t.Fatalf("unexpected like metric: %+v", metrics.Like)
	}
	if !metrics.Comment.Present || metrics.Comment.Value != 56 {
		t.Fatalf("unexpected comment metric: %+v", metrics.Comment)
	}
	if !metrics.Share.Present || metrics.Share.Value != 78 {
		t.Fatalf("unexpected share metric: %+v", metrics.Share)
	}
	if !metrics.Collect.Present || metrics.Collect.Value != 90 {
		t.Fatalf("unexpected collect metric: %+v", metrics.Collect)
	}
}

func TestEngagementMetricsMissingNativeMetricStaysUnknown(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "feed-1",
		"likeCount": 0
	}`)
	metrics := engagement_metrics_from_fetch(raw)
	if !metrics.Like.Present || metrics.Like.Value != 0 {
		t.Fatalf("explicit zero like count must stay observed: %+v", metrics.Like)
	}
	if metrics.Comment.Present || metrics.Share.Present || metrics.Collect.Present {
		t.Fatalf("missing metrics must remain unknown: %+v", metrics)
	}
}

func TestEngagementMetricsFromSharedFeed(t *testing.T) {
	raw := json.RawMessage(`{
		"data": {
			"feedInfo": {
				"likeCountFmt": "1.2万",
				"commentCountFmt": "345",
				"forwardCountFmt": "67",
				"favCountFmt": "890"
			}
		}
	}`)
	metrics := engagement_metrics_from_fetch(raw)
	if !metrics.Like.Present || metrics.Like.Value != 12000 {
		t.Fatalf("unexpected like metric: %+v", metrics.Like)
	}
	if !metrics.Comment.Present || metrics.Comment.Value != 345 {
		t.Fatalf("unexpected comment metric: %+v", metrics.Comment)
	}
	if !metrics.Share.Present || metrics.Share.Value != 67 {
		t.Fatalf("unexpected share metric: %+v", metrics.Share)
	}
	if !metrics.Collect.Present || metrics.Collect.Value != 890 {
		t.Fatalf("unexpected collect metric: %+v", metrics.Collect)
	}
}

func TestEngagementMetricsPreferObjectFavInfo(t *testing.T) {
	raw := json.RawMessage(`{
		"data": {
			"object": {
				"objectExtend": {
					"favInfo": {
						"likeCount": 9876,
						"favCount": 4321
					}
				}
			},
			"commentCount": 123
		}
	}`)
	metrics := engagement_metrics_from_fetch(raw)
	if metrics.Like.Value != 9876 || !metrics.Like.Present {
		t.Fatalf("unexpected like metric: %+v", metrics.Like)
	}
	if metrics.Collect.Value != 4321 || !metrics.Collect.Present {
		t.Fatalf("unexpected collect metric: %+v", metrics.Collect)
	}
	if metrics.Comment.Value != 123 || !metrics.Comment.Present {
		t.Fatalf("unexpected comment metric: %+v", metrics.Comment)
	}
}

func TestEngagementMetricsIgnoreCommentLevelLikes(t *testing.T) {
	raw := json.RawMessage(`{
		"data": {
			"object": {"id": "feed-1"},
			"commentCount": 2,
			"commentInfo": [
				{"id": "comment-1", "likeCount": 9999},
				{"id": "comment-2", "likeCount": 8888}
			]
		}
	}`)
	metrics := engagement_metrics_from_fetch(raw)
	if metrics.Like.Present {
		t.Fatalf("comment like counter must not become feed likes: %+v", metrics.Like)
	}
	if !metrics.Comment.Present || metrics.Comment.Value != 2 {
		t.Fatalf("unexpected comment metric: %+v", metrics.Comment)
	}
}

func TestApplyEngagementMetricsMarksObservedZero(t *testing.T) {
	content := &model.Content{Metadata: `{"key":"decode-key"}`}
	apply_engagement_metrics(content, engagement_metrics{
		Like:    engagement_metric{Value: 0, Present: true},
		Comment: engagement_metric{Value: 12, Present: true},
	})
	if content.LikeCount != 0 || content.CommentCount != 12 {
		t.Fatalf("unexpected counters: like=%d comment=%d", content.LikeCount, content.CommentCount)
	}
	var metadata map[string]any
	if err := json.Unmarshal([]byte(content.Metadata), &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if metadata["key"] != "decode-key" {
		t.Fatalf("existing metadata was lost: %+v", metadata)
	}
	observed, ok := metadata[model.ContentMetadataEngagementObservedKey].([]any)
	if !ok || len(observed) != 2 || observed[0] != "like" || observed[1] != "comment" {
		t.Fatalf("unexpected observed metadata: %+v", metadata[model.ContentMetadataEngagementObservedKey])
	}
}
