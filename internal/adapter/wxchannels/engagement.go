package wxchannelsadapter

import (
	"encoding/json"
	"strconv"
	"strings"

	"wx_channel/pkg/scraper/wxchannels"
	"wx_channel/pkg/util"
)

func now_seconds_safe() int64 {
	return int64(util.NowSeconds())
}

// parse_display_count converts WeChat display counters such as
// "1234", "1.2万", "3.4w", "878", "10万+" into an integer.
func parse_display_count(raw string) int64 {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0
	}
	s = strings.TrimSuffix(s, "+")
	s = strings.ReplaceAll(s, ",", "")
	lower := strings.ToLower(s)
	multiplier := int64(1)
	switch {
	case strings.HasSuffix(lower, "万") || strings.HasSuffix(lower, "w"):
		multiplier = 10000
		lower = strings.TrimSuffix(lower, "万")
		lower = strings.TrimSuffix(lower, "w")
	case strings.HasSuffix(lower, "亿"):
		multiplier = 100000000
		lower = strings.TrimSuffix(lower, "亿")
	case strings.HasSuffix(lower, "k"):
		multiplier = 1000
		lower = strings.TrimSuffix(lower, "k")
	}
	lower = strings.TrimSpace(lower)
	if lower == "" {
		return 0
	}
	if v, err := strconv.ParseFloat(lower, 64); err == nil {
		return int64(v * float64(multiplier))
	}
	if v, err := strconv.ParseInt(lower, 10, 64); err == nil {
		return v * multiplier
	}
	return 0
}

// apply_shared_engagement copies shared-profile display counters onto a channels object.
func apply_shared_engagement(obj *wxchannels.ChannelsObject, feed_info wxchannels.SharedFeedinfo) {
	if obj == nil {
		return
	}
	if obj.LikeCount == 0 {
		obj.LikeCount = parse_display_count(feed_info.Likecountfmt)
	}
	if obj.ForwardCount == 0 {
		obj.ForwardCount = parse_display_count(feed_info.Forwardcountfmt)
	}
	if obj.CommentCount == 0 {
		obj.CommentCount = parse_display_count(feed_info.Commentcountfmt)
	}
	if obj.FavCount == 0 {
		obj.FavCount = parse_display_count(feed_info.Favcountfmt)
	}
	if obj.CreateTime == 0 {
		obj.CreateTime = feed_info.Createtime
	}
}

// apply_profile_comment_count fills comment count from feed profile data when the object lacks it.
func apply_profile_comment_count(obj *wxchannels.ChannelsObject, comment_count int) {
	if obj == nil || obj.CommentCount != 0 || comment_count <= 0 {
		return
	}
	obj.CommentCount = int64(comment_count)
}

// engagement_metadata builds a stable metadata map for download-task storage.
func engagement_metadata(eng wxchannels.Engagement, captured_at int64) map[string]any {
	return map[string]any{
		"play_count":           eng.PlayCount,
		"play_count_available": eng.PlayCountAvailable,
		"like_count":           eng.LikeCount,
		"comment_count":        eng.CommentCount,
		"share_count":          eng.ShareCount,
		"collect_count":        eng.CollectCount,
		"publish_time":         eng.PublishTime,
		"captured_at":          captured_at,
	}
}

// build_task_metadata_json builds download-task metadata including engagement counters.
func build_task_metadata_json(obj *wxchannels.ChannelsObject, content_type string, author string) string {
	if obj == nil {
		return ""
	}
	eng := obj.ExtractEngagement()
	payload := map[string]any{
		"platform":     PlatformID,
		"id":           obj.ID,
		"content_type": content_type,
		"author":       author,
		"download_at":  now_seconds_safe(),
		"engagement":   engagement_metadata(eng, now_seconds_safe()),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}
