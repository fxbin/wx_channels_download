package api

import (
	"reflect"
	"testing"
)

func TestNormalizeContentIDs(t *testing.T) {
	input := []string{
		" wxchannels:first ",
		"",
		"wxchannels:second",
		"wxchannels:first",
		"   ",
		"wxchannels:third",
	}
	want := []string{
		"wxchannels:first",
		"wxchannels:second",
		"wxchannels:third",
	}

	got := normalize_content_ids(input)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalize_content_ids() = %#v, want %#v", got, want)
	}
}

func TestNormalizeContentIDsKeepsOnlyExplicitValues(t *testing.T) {
	got := normalize_content_ids([]string{"content:a", "content:c"})
	want := []string{"content:a", "content:c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalize_content_ids() = %#v, want %#v", got, want)
	}
}
