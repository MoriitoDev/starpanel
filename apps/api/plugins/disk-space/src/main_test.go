package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func get(t *testing.T, handler http.Handler, path string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET %s answered something that is not JSON: %v (%s)", path, err, recorder.Body.String())
	}
	return recorder, body
}

func TestHealthAnswersOK(t *testing.T) {
	recorder, body := get(t, newServer().routes(), "/health")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status is %d, want 200", recorder.Code)
	}
	if body["status"] != "ok" {
		t.Errorf("status field is %v, want ok", body["status"])
	}
	if body["version"] != version {
		t.Errorf("version is %v, want %s", body["version"], version)
	}
}

// The card polls this, so it has to answer on a machine with no /proc as well
// as one with: a build that cannot measure says so and the handler still runs.
func TestSummaryAnswersTheShapeTheCardReads(t *testing.T) {
	recorder, body := get(t, newServer().routes(), "/summary")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status is %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	for _, key := range []string{"disks", "reclaimableBytes", "reclaimableIsEstimate", "categories", "warnings", "indexAge"} {
		if _, ok := body[key]; !ok {
			t.Errorf("the answer has no %q", key)
		}
	}
	if body["reclaimableIsEstimate"] != true {
		t.Error("reclaimableIsEstimate is false, but no Category has measured anything yet")
	}
}

// All six Categories exist from the first day, even with no scanner behind
// them: the wire shape is what the dialog, the Plan and the apply step key off,
// and a field added later is a field every client already ignores.
func TestSummaryListsEveryCategoryInAStableOrder(t *testing.T) {
	_, body := get(t, newServer().routes(), "/summary")
	categories, ok := body["categories"].([]any)
	if !ok {
		t.Fatalf("categories is %T, want a list", body["categories"])
	}
	if len(categories) != len(categoryOrder) {
		t.Fatalf("got %d categories, want %d", len(categories), len(categoryOrder))
	}
	for index, raw := range categories {
		category := raw.(map[string]any)
		want := categoryOrder[index]
		if category["id"] != string(want) {
			t.Errorf("category %d is %v, want %s", index, category["id"], want)
		}
		if category["title"] == "" || category["rule"] == "" {
			t.Errorf("category %s has no title or rule for a person to read", want)
		}
	}
}

// A large file is big and not dead: its Category is never tickable, and it says
// so in one field rather than leaving the widget to remember the rule.
func TestLargeFilesIsReviewOnly(t *testing.T) {
	_, body := get(t, newServer().routes(), "/summary")
	for _, raw := range body["categories"].([]any) {
		category := raw.(map[string]any)
		if category["id"] != "large-files" {
			continue
		}
		if category["tickable"] != false {
			t.Error("large-files is tickable; it must never offer to remove anything")
		}
		if category["reviewOnly"] != true {
			t.Error("large-files does not say it is review-only")
		}
		return
	}
	t.Fatal("large-files is missing from the listing")
}

// Every Disk carries the four fields the card renders, and the percentage
// agrees with the bytes it came from.
func TestSummaryDisksCarryConsistentNumbers(t *testing.T) {
	if sourceName() != "procfs" {
		t.Skip("this build does not read /proc")
	}
	_, body := get(t, newServer().routes(), "/summary")
	disks, _ := body["disks"].([]any)
	for _, raw := range disks {
		disk := raw.(map[string]any)
		for _, key := range []string{"name", "fs", "mount", "totalBytes", "usedBytes", "freeBytes", "usedPercent"} {
			if _, ok := disk[key]; !ok {
				t.Errorf("a Disk has no %q: %v", key, disk)
			}
		}
		total := disk["totalBytes"].(float64)
		used := disk["usedBytes"].(float64)
		percent := disk["usedPercent"].(float64)
		if total > 0 {
			want := float64(int((used/total*100)*10+0.5)) / 10
			if percent != want {
				t.Errorf("usedPercent is %v but the bytes say %v", percent, want)
			}
		}
	}
}

func TestShortDurationReadsLikeAPersonWroteIt(t *testing.T) {
	cases := []struct {
		elapsed time.Duration
		want    string
	}{
		{0, "0s"},
		{42 * time.Second, "42s"},
		{60 * time.Second, "1m"},
		{59 * time.Minute, "59m"},
		{time.Hour, "1h"},
	}
	for _, c := range cases {
		if got := shortDuration(c.elapsed); got != c.want {
			t.Errorf("shortDuration(%s) = %q, want %q", c.elapsed, got, c.want)
		}
	}
}
