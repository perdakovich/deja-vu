package inspect

import (
	"encoding/json"
	"testing"
)

func TestCompactionWireDistinction(t *testing.T) {
	// 1. Test canonical compaction summary with {start, end} and surfaceOp replace with {startSeq, endSeq}
	rawSummaryCanonical := []byte(`{
		"type": "compaction/summary",
		"seq": 42,
		"surfaceOp": {
			"op": "replace",
			"startSeq": 10,
			"endSeq": 35
		},
		"data": {
			"compactionId": "cmp-canonical",
			"summary": "Canonical compaction summary text",
			"shadowedRange": {
				"start": 10,
				"end": 35
			},
			"shadowedSeqs": [10, 11, 12, 13],
			"shadowedTokenCount": 3500
		}
	}`)

	var evCanonical DeepSeekEvent
	if err := json.Unmarshal(rawSummaryCanonical, &evCanonical); err != nil {
		t.Fatalf("failed to unmarshal canonical summary event: %v", err)
	}

	itemCanonical := buildTimelineItem(evCanonical, "session.v3.jsonl", nil, nil)

	// Verify surfaceOp distinction: uses startSeq / endSeq
	if itemCanonical.SurfaceOp != "replace" {
		t.Errorf("SurfaceOp = %q, want replace", itemCanonical.SurfaceOp)
	}
	if itemCanonical.ReplacesRange == nil {
		t.Fatal("ReplacesRange is nil")
	}
	if (*itemCanonical.ReplacesRange)[0] != 10 || (*itemCanonical.ReplacesRange)[1] != 35 {
		t.Errorf("ReplacesRange = %v, want [10, 35]", *itemCanonical.ReplacesRange)
	}

	// Verify compaction fact distinction: uses start / end
	if itemCanonical.Compaction == nil {
		t.Fatal("Compaction fact is nil")
	}
	if itemCanonical.Compaction.CompactionID != "cmp-canonical" {
		t.Errorf("CompactionID = %q, want cmp-canonical", itemCanonical.Compaction.CompactionID)
	}
	if itemCanonical.Compaction.ShadowedRange == nil {
		t.Fatal("Compaction.ShadowedRange is nil")
	}
	if (*itemCanonical.Compaction.ShadowedRange)[0] != 10 || (*itemCanonical.Compaction.ShadowedRange)[1] != 35 {
		t.Errorf("Compaction.ShadowedRange = %v, want [10, 35]", *itemCanonical.Compaction.ShadowedRange)
	}
	if itemCanonical.Compaction.ShadowedTokenCount != 3500 {
		t.Errorf("ShadowedTokenCount = %d, want 3500", itemCanonical.Compaction.ShadowedTokenCount)
	}

	// 2. Test compaction prune with {start, end}
	rawPruneCanonical := []byte(`{
		"type": "compaction/prune",
		"seq": 50,
		"data": {
			"shadowedRange": {
				"start": 36,
				"end": 49
			},
			"shadowedSeqs": [36, 37, 38],
			"shadowedTokenCount": 1200
		}
	}`)

	var evPrune DeepSeekEvent
	if err := json.Unmarshal(rawPruneCanonical, &evPrune); err != nil {
		t.Fatalf("failed to unmarshal canonical prune event: %v", err)
	}

	itemPrune := buildTimelineItem(evPrune, "session.v3.jsonl", nil, nil)
	if itemPrune.Compaction == nil {
		t.Fatal("Prune Compaction fact is nil")
	}
	if itemPrune.Compaction.Kind != "prune" {
		t.Errorf("Compaction.Kind = %q, want prune", itemPrune.Compaction.Kind)
	}
	if itemPrune.Compaction.ShadowedRange == nil {
		t.Fatal("Prune ShadowedRange is nil")
	}
	if (*itemPrune.Compaction.ShadowedRange)[0] != 36 || (*itemPrune.Compaction.ShadowedRange)[1] != 49 {
		t.Errorf("Prune ShadowedRange = %v, want [36, 49]", *itemPrune.Compaction.ShadowedRange)
	}
	if itemPrune.Compaction.ShadowedTokenCount != 1200 {
		t.Errorf("Prune ShadowedTokenCount = %d, want 1200", itemPrune.Compaction.ShadowedTokenCount)
	}

	// 3. Test alternate array format [start, end] tolerance
	rawSummaryArray := []byte(`{
		"type": "compaction/summary",
		"seq": 60,
		"data": {
			"compactionId": "cmp-array",
			"summary": "Array compaction summary",
			"shadowedRange": [50, 59],
			"shadowedTokenCount": 800
		}
	}`)

	var evArray DeepSeekEvent
	if err := json.Unmarshal(rawSummaryArray, &evArray); err != nil {
		t.Fatalf("failed to unmarshal array summary event: %v", err)
	}

	itemArray := buildTimelineItem(evArray, "session.v3.jsonl", nil, nil)
	if itemArray.Compaction == nil || itemArray.Compaction.ShadowedRange == nil {
		t.Fatal("Array ShadowedRange failed to parse")
	}
	if (*itemArray.Compaction.ShadowedRange)[0] != 50 || (*itemArray.Compaction.ShadowedRange)[1] != 59 {
		t.Errorf("Array ShadowedRange = %v, want [50, 59]", *itemArray.Compaction.ShadowedRange)
	}
}
