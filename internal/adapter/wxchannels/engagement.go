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

// Typical public engagement rates for WeChat Channels-style short video.
// Each value is engagement / play_count (plays must be at least the count of any
// single public action). Rates are intentionally conservative mid-points of
// commonly observed short-video ranges and are tunable via PlayEstimateRates.
type PlayEstimateRates struct {
	Like    float64 // default 0.025 → 2.5% of viewers tap like
	Comment float64 // default 0.001 → 0.1% comment
	Share   float64 // default 0.004 → 0.4% forward/share
	Collect float64 // default 0.003 → 0.3% favorite/collect
}

// DefaultPlayEstimateRates are the built-in conversion rates used when callers
// do not override them.
var DefaultPlayEstimateRates = PlayEstimateRates{
	Like:    0.025,
	Comment: 0.001,
	Share:   0.004,
	Collect: 0.003,
}

// PlayEstimate is a best-effort play/view count derived from public engagement.
type PlayEstimate struct {
	// PlayCount is the combined estimate.
	PlayCount int64
	// Signals maps each input signal name to its independent estimate.
	Signals map[string]int64
	// Method documents how PlayCount was combined.
	Method string
	// Rates used for the calculation.
	Rates PlayEstimateRates
}

func estimate_from_signal(count int64, rate float64) int64 {
	if count <= 0 || rate <= 0 {
		return 0
	}
	return int64(float64(count)/rate + 0.5)
}

// EstimatePlayCount derives a play/view estimate from public engagement counters.
// It combines per-signal estimates with their median (robust to a single skewed
// action type) and enforces a lower bound: plays cannot be below the largest
// public action count times a small multiplier.
func EstimatePlayCount(eng wxchannels.Engagement, rates PlayEstimateRates) PlayEstimate {
	if rates == (PlayEstimateRates{}) {
		rates = DefaultPlayEstimateRates
	}
	signals := map[string]int64{}
	if v := estimate_from_signal(eng.LikeCount, rates.Like); v > 0 {
		signals["like_count"] = v
	}
	if v := estimate_from_signal(eng.CommentCount, rates.Comment); v > 0 {
		signals["comment_count"] = v
	}
	if v := estimate_from_signal(eng.ShareCount, rates.Share); v > 0 {
		signals["share_count"] = v
	}
	if v := estimate_from_signal(eng.CollectCount, rates.Collect); v > 0 {
		signals["collect_count"] = v
	}
	if len(signals) == 0 {
		return PlayEstimate{Rates: rates, Method: "no_engagement"}
	}

	values := make([]int64, 0, len(signals))
	for _, v := range signals {
		values = append(values, v)
	}
	// insertion sort is fine for at most four values
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
	var mid int64
	if n := len(values); n%2 == 1 {
		mid = values[n/2]
	} else {
		mid = (values[n/2-1] + values[n/2]) / 2
	}

	// Lower bound: plays are at least the largest single action.
	max_action := eng.LikeCount
	if eng.CommentCount > max_action {
		max_action = eng.CommentCount
	}
	if eng.ShareCount > max_action {
		max_action = eng.ShareCount
	}
	if eng.CollectCount > max_action {
		max_action = eng.CollectCount
	}
	floor := max_action * 5
	final := mid
	if floor > final {
		final = floor
	}
	return PlayEstimate{
		PlayCount: final,
		Signals:   signals,
		Method:    "median_of_signal_estimates_with_floor",
		Rates:     rates,
	}
}

// WithPlayEstimate fills estimated play count when the platform did not expose one.
func WithPlayEstimate(eng wxchannels.Engagement, rates PlayEstimateRates) wxchannels.Engagement {
	if eng.PlayCountSource == "measured" && eng.PlayCountAvailable {
		return eng
	}
	est := EstimatePlayCount(eng, rates)
	if est.PlayCount <= 0 {
		return eng
	}
	eng.PlayCount = est.PlayCount
	eng.PlayCountAvailable = true
	eng.PlayCountSource = "estimated"
	eng.EstimatedPlayCount = est.PlayCount
	eng.PlayEstimateSignals = est.Signals
	return eng
}

// engagement_metadata builds a stable metadata map for download-task storage.
func engagement_metadata(eng wxchannels.Engagement, captured_at int64) map[string]any {
	source := eng.PlayCountSource
	if source == "" {
		if eng.PlayCountAvailable {
			source = "measured"
		} else {
			source = "none"
		}
	}
	payload := map[string]any{
		"play_count":           eng.PlayCount,
		"play_count_available": eng.PlayCountAvailable,
		"play_count_source":    source,
		"like_count":           eng.LikeCount,
		"comment_count":        eng.CommentCount,
		"share_count":          eng.ShareCount,
		"collect_count":        eng.CollectCount,
		"publish_time":         eng.PublishTime,
		"captured_at":          captured_at,
	}
	if eng.PlayCountSource == "estimated" {
		payload["estimated_play_count"] = eng.EstimatedPlayCount
		payload["play_estimate_signals"] = eng.PlayEstimateSignals
		payload["play_estimate_rates"] = map[string]float64{
			"like":    DefaultPlayEstimateRates.Like,
			"comment": DefaultPlayEstimateRates.Comment,
			"share":   DefaultPlayEstimateRates.Share,
			"collect": DefaultPlayEstimateRates.Collect,
		}
	}
	return payload
}

// build_task_metadata_json builds download-task metadata including engagement counters.
func build_task_metadata_json(obj *wxchannels.ChannelsObject, content_type string, author string) string {
	if obj == nil {
		return ""
	}
	eng := WithPlayEstimate(obj.ExtractEngagement(), DefaultPlayEstimateRates)
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
