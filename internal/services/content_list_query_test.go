package services

import "testing"

func TestNormalizeContentSort(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		wantBy    string
		wantOrder string
		wantErr   bool
	}{
		{name: "defaults", wantBy: ContentSortCreatedAt, wantOrder: ContentSortOrderDesc},
		{name: "publish asc", sortBy: ContentSortPublishTime, sortOrder: ContentSortOrderAsc, wantBy: ContentSortPublishTime, wantOrder: ContentSortOrderAsc},
		{name: "likes desc", sortBy: ContentSortLikeCount, sortOrder: ContentSortOrderDesc, wantBy: ContentSortLikeCount, wantOrder: ContentSortOrderDesc},
		{name: "reject field", sortBy: "title", sortOrder: ContentSortOrderDesc, wantErr: true},
		{name: "reject direction", sortBy: ContentSortPublishTime, sortOrder: "sideways", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotBy, gotOrder, err := normalize_content_sort(test.sortBy, test.sortOrder)
			if test.wantErr {
				if err == nil {
					t.Fatalf("expected error, got sort=%s order=%s", gotBy, gotOrder)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotBy != test.wantBy || gotOrder != test.wantOrder {
				t.Fatalf("got (%s,%s), want (%s,%s)", gotBy, gotOrder, test.wantBy, test.wantOrder)
			}
		})
	}
}

func TestContentOrderClauseUsesWhitelistedColumns(t *testing.T) {
	cases := []struct {
		by    string
		order string
		want  string
	}{
		{ContentSortCreatedAt, ContentSortOrderDesc, "content.created_at DESC, content.id DESC"},
		{ContentSortPublishTime, ContentSortOrderAsc, "CASE WHEN content.publish_time IS NULL THEN 1 ELSE 0 END ASC, content.publish_time ASC, content.id ASC"},
		{ContentSortLikeCount, ContentSortOrderDesc, "content.like_count DESC, COALESCE(content.publish_time, 0) DESC, content.id DESC"},
	}
	for _, test := range cases {
		if got := content_order_clause(test.by, test.order); got != test.want {
			t.Fatalf("content_order_clause(%q,%q) = %q, want %q", test.by, test.order, got, test.want)
		}
	}
}
