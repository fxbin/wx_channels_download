package services

import (
	"fmt"
	"strings"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
)

const (
	ContentSortCreatedAt   = "created_at"
	ContentSortPublishTime = "publish_time"
	ContentSortLikeCount   = "like_count"
	ContentSortOrderAsc    = "asc"
	ContentSortOrderDesc   = "desc"
)

type ContentRankedListOptions struct {
	ContentListOptions
	MinLikeCount *int64
	SortBy       string
	SortOrder    string
}

type ContentRankedListItem struct {
	ContentListItem
	ViewCount    int64 `json:"view_count"`
	LikeCount    int64 `json:"like_count"`
	CommentCount int64 `json:"comment_count"`
	ShareCount   int64 `json:"share_count"`
	CollectCount int64 `json:"collect_count"`
}

type ContentRankedListResult struct {
	List     []ContentRankedListItem `json:"list"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

func normalize_content_sort(sort_by, sort_order string) (string, string, error) {
	sort_by = strings.ToLower(strings.TrimSpace(sort_by))
	if sort_by == "" {
		sort_by = ContentSortCreatedAt
	}
	switch sort_by {
	case ContentSortCreatedAt, ContentSortPublishTime, ContentSortLikeCount:
	default:
		return "", "", fmt.Errorf("sort_by must be one of: created_at, publish_time, like_count")
	}

	sort_order = strings.ToLower(strings.TrimSpace(sort_order))
	if sort_order == "" {
		sort_order = ContentSortOrderDesc
	}
	if sort_order != ContentSortOrderAsc && sort_order != ContentSortOrderDesc {
		return "", "", fmt.Errorf("sort_order must be asc or desc")
	}
	return sort_by, sort_order, nil
}

func content_order_clause(sort_by, sort_order string) string {
	direction := "DESC"
	if sort_order == ContentSortOrderAsc {
		direction = "ASC"
	}
	switch sort_by {
	case ContentSortPublishTime:
		return "CASE WHEN content.publish_time IS NULL THEN 1 ELSE 0 END ASC, content.publish_time " + direction + ", content.id " + direction
	case ContentSortLikeCount:
		return "content.like_count " + direction + ", COALESCE(content.publish_time, 0) DESC, content.id DESC"
	default:
		return "content.created_at " + direction + ", content.id " + direction
	}
}

// ListContentsRanked extends ListContents with a strict sort whitelist and
// engagement filtering while preserving the existing response shape.
func (s *ContentService) ListContentsRanked(options ContentRankedListOptions) (*ContentRankedListResult, error) {
	if s.db == nil {
		return nil, ErrDBNotInitialized
	}

	scope := strings.ToLower(strings.TrimSpace(options.Scope))
	if scope == "" {
		scope = ContentListScopeTask
	}
	if scope != ContentListScopeAll && scope != ContentListScopeTask {
		return nil, fmt.Errorf("unsupported content list scope: %s", scope)
	}
	if options.StartAt != nil && *options.StartAt < 0 {
		return nil, fmt.Errorf("start_at must be non-negative")
	}
	if options.EndAt != nil && *options.EndAt < 0 {
		return nil, fmt.Errorf("end_at must be non-negative")
	}
	if options.StartAt != nil && options.EndAt != nil && *options.StartAt >= *options.EndAt {
		return nil, fmt.Errorf("start_at must be less than end_at")
	}
	if options.MinLikeCount != nil && *options.MinLikeCount < 0 {
		return nil, fmt.Errorf("min_like_count must be non-negative")
	}

	sort_by, sort_order, err := normalize_content_sort(options.SortBy, options.SortOrder)
	if err != nil {
		return nil, err
	}

	page := options.Page
	if page < 1 {
		page = 1
	}
	page_size := options.PageSize
	if page_size < 1 {
		page_size = 20
	}
	if page_size > 200 {
		page_size = 200
	}
	offset := (page - 1) * page_size
	if options.Offset != nil && *options.Offset >= 0 {
		offset = *options.Offset
	}

	build_query := func() *gorm.DB {
		query := s.db.Model(&model.Content{})
		if scope == ContentListScopeTask {
			query = query.Where(`EXISTS (
				SELECT 1 FROM download_task
				WHERE download_task.content_id = content.id
					AND download_task.deleted_at IS NULL
			)`)
		}
		if content_type := strings.TrimSpace(options.Type); content_type != "" {
			query = query.Where("content.type = ?", content_type)
		}
		if account_id := strings.TrimSpace(options.AccountID); account_id != "" {
			query = query.
				Joins("JOIN content_account ON content_account.content_id = content.id").
				Where("content_account.account_id = ?", account_id)
		}
		if keyword := strings.TrimSpace(options.Keyword); keyword != "" {
			pattern := "%" + keyword + "%"
			query = query.Where("content.title LIKE ? OR content.description LIKE ?", pattern, pattern)
		}
		// Preserve the pre-existing start_at/end_at contract: these filter the
		// archive creation time rather than the platform publish time.
		if options.StartAt != nil {
			query = query.Where("content.created_at >= ?", *options.StartAt)
		}
		if options.EndAt != nil {
			query = query.Where("content.created_at < ?", *options.EndAt)
		}
		if options.MinLikeCount != nil {
			query = query.Where("content.like_count >= ?", *options.MinLikeCount)
		}
		return query
	}

	var total int64
	if err := build_query().Distinct("content.id").Count(&total).Error; err != nil {
		return nil, err
	}

	var contents []model.Content
	if err := build_query().
		Distinct("content.*").
		Order(content_order_clause(sort_by, sort_order)).
		Limit(page_size).
		Offset(offset).
		Find(&contents).Error; err != nil {
		return nil, err
	}

	content_ids := make([]string, 0, len(contents))
	for _, content := range contents {
		content_ids = append(content_ids, content.Id)
	}
	accounts_by_content_id, influencers_by_content_id, download_tasks_by_content_id, _, err := s.load_content_relations(content_ids, false)
	if err != nil {
		return nil, err
	}
	file_counts_by_content_id, err := s.content_file_counts(content_ids)
	if err != nil {
		return nil, err
	}

	list := make([]ContentRankedListItem, 0, len(contents))
	for _, content := range contents {
		publish_time := int64(0)
		if content.PublishTime != nil {
			publish_time = *content.PublishTime
		}
		accounts := accounts_by_content_id[content.Id]
		if accounts == nil {
			accounts = make([]ContentAccountRecord, 0)
		}
		influencers := influencers_by_content_id[content.Id]
		if influencers == nil {
			influencers = make([]ContentInfluencerRecord, 0)
		}
		download_tasks := download_tasks_by_content_id[content.Id]
		if download_tasks == nil {
			download_tasks = make([]ContentDownloadTaskRecord, 0)
		}

		list = append(list, ContentRankedListItem{
			ContentListItem: ContentListItem{
				ID:            content.Id,
				PlatformID:    content.PlatformId,
				Type:          content.Type,
				Subtype:       content.Subtype,
				ExternalID:    content.ExternalId,
				ExternalID2:   content.ExternalId2,
				ExternalID3:   content.ExternalId3,
				Title:         content.Title,
				Description:   content.Description,
				URL:           content.URL,
				SourceURL:     content.SourceURL,
				CoverURL:      content.CoverURL,
				CoverWidth:    content.CoverWidth,
				CoverHeight:   content.CoverHeight,
				PublishTime:   publish_time,
				Accounts:      accounts,
				Influencers:   influencers,
				DownloadTasks: download_tasks,
				FileCount:     file_counts_by_content_id[content.Id],
			},
			ViewCount:    content.ViewCount,
			LikeCount:    content.LikeCount,
			CommentCount: content.CommentCount,
			ShareCount:   content.ShareCount,
			CollectCount: content.CollectCount,
		})
	}

	return &ContentRankedListResult{List: list, Total: total, Page: page, PageSize: page_size}, nil
}

func (s *ContentService) content_file_counts(content_ids []string) (map[string]int64, error) {
	counts := make(map[string]int64, len(content_ids))
	if len(content_ids) == 0 {
		return counts, nil
	}
	type count_row struct {
		ContentID string `gorm:"column:content_id"`
		Count     int64  `gorm:"column:count"`
	}
	var rows []count_row
	if err := s.db.Model(&model.DownloadResource{}).
		Select("content_id, COUNT(*) AS count").
		Where("content_id IN ? AND deleted_at IS NULL", content_ids).
		Group("content_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.ContentID] = row.Count
	}

	var embedded_rows []count_row
	if err := s.db.Table("content_relation AS relation").
		Select("relation.source_content_id AS content_id, COUNT(download_resource.id) AS count").
		Joins("JOIN content AS parent_content ON parent_content.id = relation.source_content_id").
		Joins("JOIN content AS embedded_content ON embedded_content.id = relation.target_content_id").
		Joins(`JOIN download_resource ON download_resource.content_id = embedded_content.id AND download_resource.deleted_at IS NULL`).
		Where(`relation.source_content_id IN ? AND relation.type = ?
			AND parent_content.type IN ? AND parent_content.deleted_at IS NULL
			AND embedded_content.type IN ? AND embedded_content.deleted_at IS NULL`,
			content_ids, model.ContentRelationContains, embedded_content_parent_types, embedded_content_media_types).
		Group("relation.source_content_id").
		Scan(&embedded_rows).Error; err != nil {
		return nil, err
	}
	for _, row := range embedded_rows {
		counts[row.ContentID] += row.Count
	}
	return counts, nil
}
