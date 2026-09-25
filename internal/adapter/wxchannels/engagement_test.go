package wxchannelsadapter

import (
	"encoding/json"
	"testing"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/scraper/wxchannels"
)

func TestParseDisplayCount(t *testing.T) {
	cases := []struct {
		raw  string
		want int64
	}{
		{"", 0},
		{"878", 878},
		{"1,234", 1234},
		{"1.2万", 12000},
		{"10万+", 100000},
		{"3.4w", 34000},
		{"2.5k", 2500},
		{"1亿", 100000000},
		{"  99  ", 99},
		{"abc", 0},
	}
	for _, c := range cases {
		if got := parse_display_count(c.raw); got != c.want {
			t.Fatalf("parse_display_count(%q)=%d want %d", c.raw, got, c.want)
		}
	}
}

func TestExtractEngagementFromObjectFields(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:           "obj-1",
		CreateTime:   1766574001,
		LikeCount:    300,
		CommentCount: 138,
		ForwardCount: 878,
		FavCount:     1435,
	}
	eng := obj.ExtractEngagement()
	if eng.LikeCount != 300 || eng.CommentCount != 138 || eng.ShareCount != 878 || eng.CollectCount != 1435 {
		t.Fatalf("unexpected engagement: %+v", eng)
	}
	if eng.PublishTime != 1766574001 {
		t.Fatalf("publish time=%d", eng.PublishTime)
	}
	if eng.PlayCountAvailable {
		t.Fatal("play count should be unavailable when not provided")
	}
}

func TestExtractEngagementFromMonotonicData(t *testing.T) {
	mono := map[string]any{
		"countInfo": map[string]any{
			"commentCount": 11,
			"likeCount":    22,
			"forwardCount": 33,
			"favCount":     44,
		},
	}
	raw, _ := json.Marshal(mono)
	obj := &wxchannels.ChannelsObject{
		ID:          "obj-2",
		ObjectExtend: &wxchannels.ObjectExtend{MonotonicData: raw},
	}
	eng := obj.ExtractEngagement()
	if eng.LikeCount != 22 || eng.CommentCount != 11 || eng.ShareCount != 33 || eng.CollectCount != 44 {
		t.Fatalf("unexpected engagement from monotonicData: %+v", eng)
	}
}

func TestExtractEngagementPlayCount(t *testing.T) {
	obj := &wxchannels.ChannelsObject{ID: "obj-3", PlayCount: 1000}
	eng := obj.ExtractEngagement()
	if !eng.PlayCountAvailable || eng.PlayCount != 1000 {
		t.Fatalf("expected play count 1000, got %+v", eng)
	}

	obj2 := &wxchannels.ChannelsObject{ID: "obj-4", ReadCount: 55}
	eng2 := obj2.ExtractEngagement()
	if !eng2.PlayCountAvailable || eng2.PlayCount != 55 {
		t.Fatalf("expected read count mapped to play count 55, got %+v", eng2)
	}
}

func TestApplySharedEngagement(t *testing.T) {
	obj := &wxchannels.ChannelsObject{ID: "shared-1"}
	feed := wxchannels.SharedFeedinfo{
		Likecountfmt:    "1.2万",
		Forwardcountfmt: "878",
		Commentcountfmt: "138",
		Favcountfmt:     "1435",
		Createtime:      1766574001,
	}
	apply_shared_engagement(obj, feed)
	if obj.LikeCount != 12000 || obj.ForwardCount != 878 || obj.CommentCount != 138 || obj.FavCount != 1435 {
		t.Fatalf("unexpected shared engagement: %+v", obj)
	}
	if obj.CreateTime != 1766574001 {
		t.Fatalf("createtime=%d", obj.CreateTime)
	}
}

func TestToContentMapsEngagement(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:           "video-1",
		ObjectNonceId: "nonce-1",
		CreateTime:   1766574001,
		LikeCount:    300,
		CommentCount: 138,
		ForwardCount: 878,
		FavCount:     1435,
		Contact: wxchannels.ChannelsContact{
			Username: "user-1",
			Nickname: "tester",
		},
		ObjectDesc: wxchannels.ChannelsObjectDesc{
			Description: "title",
			MediaType:   wxchannels.MediaTypeVideo,
			Media: []wxchannels.ChannelsMediaItem{{
				URL:       "https://example.com/video.mp4",
				URLToken:  "?token=1",
				DecodeKey: "12345",
				ThumbUrl:  "https://example.com/cover.jpg",
				Width:     720,
				Height:    1280,
				FileSize:  2048,
				VideoPlayLen: 42,
			}},
		},
	}
	content, detail, err := ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent error: %v", err)
	}
	if content.LikeCount != 300 || content.CommentCount != 138 || content.ShareCount != 878 || content.CollectCount != 1435 {
		t.Fatalf("content engagement mismatch: %+v", content)
	}
	if content.PublishTime == nil || *content.PublishTime != 1766574001 {
		t.Fatalf("publish time mismatch: %+v", content.PublishTime)
	}
	if content.Type != "video" {
		t.Fatalf("content type=%s", content.Type)
	}
	video, ok := detail.(*model.ContentVideo)
	if !ok {
		t.Fatalf("expected ContentVideo detail, got %T", detail)
	}
	if video.PlayTimes != 0 {
		t.Fatalf("play times should stay 0 when unavailable, got %d", video.PlayTimes)
	}
}

func TestToContentMapsPlayCountWhenAvailable(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:           "video-2",
		ObjectNonceId: "nonce-2",
		CreateTime:   1766574001,
		PlayCount:    9999,
		LikeCount:    1,
		ObjectDesc: wxchannels.ChannelsObjectDesc{
			Description: "title",
			MediaType:   wxchannels.MediaTypeVideo,
			Media: []wxchannels.ChannelsMediaItem{{
				URL:          "https://example.com/v.mp4",
				URLToken:     "",
				DecodeKey:    "1",
				VideoPlayLen: 10,
			}},
		},
	}
	content, detail, err := ToContent(obj)
	if err != nil {
		t.Fatalf("ToContent error: %v", err)
	}
	if content.ViewCount != 9999 {
		t.Fatalf("view count=%d want 9999", content.ViewCount)
	}
	video, ok := detail.(*model.ContentVideo)
	if !ok {
		t.Fatalf("expected ContentVideo, got %T", detail)
	}
	if video.PlayTimes != 9999 {
		t.Fatalf("play times=%d want 9999", video.PlayTimes)
	}
}

func TestBuildTaskMetadataJSONIncludesEngagement(t *testing.T) {
	obj := &wxchannels.ChannelsObject{
		ID:           "video-3",
		CreateTime:   1766574001,
		LikeCount:    7,
		CommentCount: 8,
		ForwardCount: 9,
		FavCount:     10,
	}
	raw := build_task_metadata_json(obj, "video", "author")
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("metadata json invalid: %v", err)
	}
	eng, ok := payload["engagement"].(map[string]any)
	if !ok {
		t.Fatalf("missing engagement in metadata: %s", raw)
	}
	if eng["like_count"].(float64) != 7 || eng["comment_count"].(float64) != 8 {
		t.Fatalf("engagement mismatch: %+v", eng)
	}
	if payload["content_type"] != "video" {
		t.Fatalf("content_type=%v", payload["content_type"])
	}
}
