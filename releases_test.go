package main

import (
	"os"
	"testing"
	"time"
)

func TestNewestUsableReleaseSelectsPublishedDLL(t *testing.T) {
	older := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	newer := older.Add(24 * time.Hour)
	items := []release{
		{TagName: "newer-no-dll", PublishedAt: newer.Add(time.Hour)},
		{TagName: "draft", PublishedAt: newer.Add(time.Hour), Draft: true, Assets: []releaseAsset{{Name: modDLLName, URL: "https://example.test/draft"}}},
		{TagName: "older", PublishedAt: older, Assets: []releaseAsset{{Name: modDLLName, URL: "https://example.test/older"}}},
		{TagName: "newest", PublishedAt: newer, Assets: []releaseAsset{{Name: modDLLName, URL: "https://example.test/newest"}}},
	}
	got, err := newestUsableRelease(items)
	if err != nil || got.TagName != "newest" {
		t.Fatalf("selected %q, %v", got.TagName, err)
	}
}

func TestPublishedReleaseDownloads(t *testing.T) {
	if os.Getenv("ALPHARING_TEST_RELEASES") == "" {
		t.Skip("set ALPHARING_TEST_RELEASES=1 for live GitHub release checks")
	}
	for _, source := range []string{sourceMegaBit, sourceKirkland} {
		t.Run(source, func(t *testing.T) {
			data, tag, hash, err := fetchVerifiedDLL(source)
			if err != nil || tag == "" || hash == "" || len(data) < 100_000 {
				t.Fatalf("downloaded %d bytes, tag %q, hash %q: %v", len(data), tag, hash, err)
			}
			identifiedSource, identifiedTag := matchPublishedDLL(hash)
			if identifiedSource != source || identifiedTag != tag {
				t.Fatalf("recognized %q %q, want %q %q", identifiedSource, identifiedTag, source, tag)
			}
		})
	}
}
