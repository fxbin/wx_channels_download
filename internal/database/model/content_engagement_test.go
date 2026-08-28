package model

import "testing"

func TestObservedContentEngagement(t *testing.T) {
	observed := observed_content_engagement(`{"key":"decode","_engagement_observed":["like","comment"]}`)
	if !observed["like"] || !observed["comment"] {
		t.Fatalf("expected like/comment to be observed: %+v", observed)
	}
	if observed["share"] {
		t.Fatalf("share should not be observed: %+v", observed)
	}
}

func TestObservedContentEngagementInvalidMetadata(t *testing.T) {
	observed := observed_content_engagement("not-json")
	if len(observed) != 0 {
		t.Fatalf("invalid metadata should return empty observation set: %+v", observed)
	}
}
