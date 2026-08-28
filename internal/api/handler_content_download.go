package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"wx_channel/internal/adapter"
	result "wx_channel/internal/apiresult"
	"wx_channel/internal/services"
)

const content_batch_download_max_items = 200

type content_batch_download_body struct {
	ContentIDs []string `json:"content_ids"`
	AutoStart  *bool    `json:"auto_start"`
}

type content_batch_download_item struct {
	ContentID string `json:"content_id"`
	Success   bool   `json:"success"`
	Skipped   bool   `json:"skipped,omitempty"`
	TaskID    int    `json:"task_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

func normalize_content_ids(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		content_id := strings.TrimSpace(value)
		if content_id == "" {
			continue
		}
		if _, exists := seen[content_id]; exists {
			continue
		}
		seen[content_id] = struct{}{}
		result = append(result, content_id)
	}
	return result
}

func marshal_fetch_result(data any) (json.RawMessage, error) {
	switch value := data.(type) {
	case json.RawMessage:
		return value, nil
	case []byte:
		return json.RawMessage(value), nil
	case string:
		return json.RawMessage(value), nil
	default:
		raw, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(raw), nil
	}
}

// handle_content_batch_download creates download tasks only for explicitly
// selected content IDs. The platform source is re-fetched at download time so
// expiring media URLs and platform-specific decode metadata remain fresh.
func (c *APIClient) handle_content_batch_download(ctx *gin.Context) {
	if c.content_service == nil || c.download_task_service == nil {
		result.Err(ctx, 500, "下载服务未初始化")
		return
	}
	var body content_batch_download_body
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "请求参数无效")
		return
	}
	content_ids := normalize_content_ids(body.ContentIDs)
	if len(content_ids) == 0 {
		result.Err(ctx, 400, "至少选择一条内容")
		return
	}
	if len(content_ids) > content_batch_download_max_items {
		result.Err(ctx, 400, fmt.Sprintf("单次最多下载 %d 条内容", content_batch_download_max_items))
		return
	}

	items := make([]content_batch_download_item, 0, len(content_ids))
	created_count := 0
	skipped_count := 0
	failed_count := 0

	for _, content_id := range content_ids {
		item_result := content_batch_download_item{ContentID: content_id}
		content_detail, err := c.content_service.GetContentDetail(content_id)
		if err != nil {
			item_result.Error = err.Error()
			items = append(items, item_result)
			failed_count++
			continue
		}

		platform_id := strings.TrimSpace(content_detail.PlatformID)
		handler := adapter.Get(platform_id)
		if handler == nil {
			item_result.Error = "不支持的平台: " + platform_id
			items = append(items, item_result)
			failed_count++
			continue
		}
		source_url := strings.TrimSpace(content_detail.SourceURL)
		if source_url == "" {
			source_url = strings.TrimSpace(content_detail.URL)
		}
		if source_url == "" {
			item_result.Error = "内容缺少可重新获取的来源地址"
			items = append(items, item_result)
			failed_count++
			continue
		}

		fetch_data, err := handler.Fetch(source_url)
		if err != nil {
			item_result.Error = "重新获取内容失败: " + err.Error()
			items = append(items, item_result)
			failed_count++
			continue
		}
		raw_content, err := marshal_fetch_result(fetch_data)
		if err != nil {
			item_result.Error = "编码平台内容失败: " + err.Error()
			items = append(items, item_result)
			failed_count++
			continue
		}

		create_result, err := c.download_task_service.CreateTask(services.CreateDownloadTaskBody{
			Platform:       platform_id,
			Content:        raw_content,
			BuildFromFetch: true,
			AutoStart:      body.AutoStart,
		})
		if err != nil {
			var duplicate_error *services.DuplicateTaskError
			if errors.As(err, &duplicate_error) {
				item_result.Success = true
				item_result.Skipped = true
				item_result.TaskID = duplicate_error.ExistingTaskID
				items = append(items, item_result)
				skipped_count++
				continue
			}
			item_result.Error = err.Error()
			items = append(items, item_result)
			failed_count++
			continue
		}

		item_result.Success = true
		if create_result != nil {
			item_result.TaskID = create_result.Task.Id
		}
		items = append(items, item_result)
		created_count++
	}

	result.Ok(ctx, gin.H{
		"items":   items,
		"total":   len(content_ids),
		"created": created_count,
		"skipped": skipped_count,
		"failed":  failed_count,
	})
}
