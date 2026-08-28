package wxchannelsadapter

import (
	"encoding/json"
	"testing"
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
