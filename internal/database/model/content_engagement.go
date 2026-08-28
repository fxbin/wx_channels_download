package model

import (
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"
)

const ContentMetadataEngagementObservedKey = "_engagement_observed"

func observed_content_engagement(metadata string) map[string]bool {
	observed := make(map[string]bool)
	metadata = strings.TrimSpace(metadata)
	if metadata == "" {
		return observed
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(metadata), &values); err != nil {
		return observed
	}
	raw, exists := values[ContentMetadataEngagementObservedKey]
	if !exists {
		return observed
	}
	items, ok := raw.([]any)
	if !ok {
		return observed
	}
	for _, item := range items {
		name, ok := item.(string)
		if !ok {
			continue
		}
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			observed[name] = true
		}
	}
	return observed
}

// BeforeSave keeps previously observed engagement counters when an adapter
// refresh does not contain that specific metric. This matters for platform
// responses whose list/detail shapes expose different counter subsets.
//
// Only wxchannels currently emits the observation marker. A genuinely
// observed zero remains writable because the marker distinguishes zero from a
// missing field.
func (c *Content) BeforeSave(tx *gorm.DB) error {
	if c == nil || tx == nil || strings.TrimSpace(c.Id) == "" || c.PlatformId != "wxchannels" {
		return nil
	}

	var existing Content
	err := tx.Session(&gorm.Session{NewDB: true, SkipHooks: true}).
		Select("id", "view_count", "like_count", "comment_count", "share_count", "collect_count").
		Where("id = ?", c.Id).
		Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	observed := observed_content_engagement(c.Metadata)
	if !observed["view"] {
		c.ViewCount = existing.ViewCount
	}
	if !observed["like"] {
		c.LikeCount = existing.LikeCount
	}
	if !observed["comment"] {
		c.CommentCount = existing.CommentCount
	}
	if !observed["share"] {
		c.ShareCount = existing.ShareCount
	}
	if !observed["collect"] {
		c.CollectCount = existing.CollectCount
	}
	return nil
}
