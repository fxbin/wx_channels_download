package wxchannelsadapter

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"unicode"

	"wx_channel/internal/database/model"
)

type engagement_metric struct {
	Value   int64
	Present bool
}

type engagement_metrics struct {
	Like    engagement_metric
	Comment engagement_metric
	Share   engagement_metric
	Collect engagement_metric
	View    engagement_metric
}

var engagement_metric_keys = map[string][]string{
	"like":    {"likecount", "likecountfmt", "likes"},
	"comment": {"commentcount", "commentcountfmt", "comments"},
	"share":   {"sharecount", "forwardcount", "forwardcountfmt"},
	"collect": {"collectcount", "favoritecount", "favcount", "favcountfmt"},
	"view":    {"viewcount", "playcount", "readcount"},
}

func engagement_metrics_from_fetch(data any) engagement_metrics {
	raw, ok := engagement_json_bytes(data)
	if !ok || len(raw) == 0 {
		return engagement_metrics{}
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return engagement_metrics{}
	}

	// Prefer feedInfo / objectExtend payloads before falling back to a bounded
	// recursive scan. This avoids accidentally picking counters from comments
	// or other related objects when the response contains several entities.
	candidates := make([]any, 0, 4)
	if root, ok := value.(map[string]any); ok {
		if data_value, ok := map_value_case_insensitive(root, "data"); ok {
			if data_map, ok := data_value.(map[string]any); ok {
				if feed_info, ok := map_value_case_insensitive(data_map, "feedInfo"); ok {
					candidates = append(candidates, feed_info)
				}
				if object_value, ok := map_value_case_insensitive(data_map, "object"); ok {
					candidates = append(candidates, object_value)
				}
				candidates = append(candidates, data_map)
			}
		}
		if feed_info, ok := map_value_case_insensitive(root, "feedInfo"); ok {
			candidates = append(candidates, feed_info)
		}
		if object_value, ok := map_value_case_insensitive(root, "object"); ok {
			candidates = append(candidates, object_value)
		}
	}
	candidates = append(candidates, value)

	return engagement_metrics{
		Like:    find_engagement_metric(candidates, engagement_metric_keys["like"]),
		Comment: find_engagement_metric(candidates, engagement_metric_keys["comment"]),
		Share:   find_engagement_metric(candidates, engagement_metric_keys["share"]),
		Collect: find_engagement_metric(candidates, engagement_metric_keys["collect"]),
		View:    find_engagement_metric(candidates, engagement_metric_keys["view"]),
	}
}

func engagement_json_bytes(data any) ([]byte, bool) {
	switch value := data.(type) {
	case nil:
		return nil, false
	case json.RawMessage:
		return value, true
	case []byte:
		return value, true
	case string:
		return []byte(value), true
	default:
		raw, err := json.Marshal(data)
		return raw, err == nil
	}
}

func find_engagement_metric(candidates []any, keys []string) engagement_metric {
	for _, candidate := range candidates {
		if metric, ok := find_metric_in_value(candidate, keys, 0); ok {
			return engagement_metric{Value: metric, Present: true}
		}
	}
	return engagement_metric{}
}

func find_metric_in_value(value any, keys []string, depth int) (int64, bool) {
	if depth > 6 || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, expected := range keys {
			for key, raw_value := range typed {
				if normalize_metric_key(key) != expected {
					continue
				}
				if count, ok := parse_engagement_count(raw_value); ok {
					return count, true
				}
			}
		}
		// Favor likely metric containers before arbitrary nested payloads.
		for _, container := range []string{"favinfo", "feedinfo", "objectextend", "statistics", "stats", "countinfo"} {
			for key, nested := range typed {
				if normalize_metric_key(key) != container {
					continue
				}
				if count, ok := find_metric_in_value(nested, keys, depth+1); ok {
					return count, true
				}
			}
		}
		for _, nested := range typed {
			if count, ok := find_metric_in_value(nested, keys, depth+1); ok {
				return count, true
			}
		}
	case []any:
		for _, nested := range typed {
			if count, ok := find_metric_in_value(nested, keys, depth+1); ok {
				return count, true
			}
		}
	}
	return 0, false
}

func normalize_metric_key(value string) string {
	var builder strings.Builder
	for _, char := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

func map_value_case_insensitive(values map[string]any, key string) (any, bool) {
	expected := normalize_metric_key(key)
	for candidate, value := range values {
		if normalize_metric_key(candidate) == expected {
			return value, true
		}
	}
	return nil, false
}

func parse_engagement_count(value any) (int64, bool) {
	switch typed := value.(type) {
	case nil:
		return 0, false
	case float64:
		if typed < 0 || math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return int64(math.Round(typed)), true
	case float32:
		if typed < 0 {
			return 0, false
		}
		return int64(math.Round(float64(typed))), true
	case int:
		if typed < 0 {
			return 0, false
		}
		return int64(typed), true
	case int64:
		if typed < 0 {
			return 0, false
		}
		return typed, true
	case json.Number:
		return parse_engagement_count_string(typed.String())
	case string:
		return parse_engagement_count_string(typed)
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return 0, false
		}
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			return parse_engagement_count_string(text)
		}
	}
	return 0, false
}

func parse_engagement_count_string(value string) (int64, bool) {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" || text == "-" || text == "--" {
		return 0, false
	}
	text = strings.ReplaceAll(text, ",", "")
	text = strings.ReplaceAll(text, " ", "")

	multiplier := float64(1)
	for _, suffix := range []struct {
		Token      string
		Multiplier float64
	}{
		{"亿", 100000000},
		{"千万", 10000000},
		{"百万", 1000000},
		{"万", 10000},
		{"w", 10000},
		{"m", 1000000},
		{"k", 1000},
	} {
		if strings.HasSuffix(text, suffix.Token) {
			multiplier = suffix.Multiplier
			text = strings.TrimSuffix(text, suffix.Token)
			break
		}
	}
	text = strings.TrimSuffix(text, "+")
	if text == "" {
		return 0, false
	}
	number, err := strconv.ParseFloat(text, 64)
	if err != nil || number < 0 || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return int64(math.Round(number * multiplier)), true
}

func mark_engagement_observed(content *model.Content, metrics engagement_metrics) {
	if content == nil {
		return
	}
	observed := make([]string, 0, 5)
	if metrics.View.Present {
		observed = append(observed, "view")
	}
	if metrics.Like.Present {
		observed = append(observed, "like")
	}
	if metrics.Comment.Present {
		observed = append(observed, "comment")
	}
	if metrics.Share.Present {
		observed = append(observed, "share")
	}
	if metrics.Collect.Present {
		observed = append(observed, "collect")
	}

	metadata := make(map[string]any)
	if raw := strings.TrimSpace(content.Metadata); raw != "" {
		_ = json.Unmarshal([]byte(raw), &metadata)
		if metadata == nil {
			metadata = make(map[string]any)
		}
	}
	metadata[model.ContentMetadataEngagementObservedKey] = observed
	if raw, err := json.Marshal(metadata); err == nil {
		content.Metadata = string(raw)
	}
}

func apply_engagement_metrics(content *model.Content, metrics engagement_metrics) {
	if content == nil {
		return
	}
	mark_engagement_observed(content, metrics)
	if metrics.Like.Present {
		content.LikeCount = metrics.Like.Value
	}
	if metrics.Comment.Present {
		content.CommentCount = metrics.Comment.Value
	}
	if metrics.Share.Present {
		content.ShareCount = metrics.Share.Value
	}
	if metrics.Collect.Present {
		content.CollectCount = metrics.Collect.Value
	}
	if metrics.View.Present {
		content.ViewCount = metrics.View.Value
	}
}
