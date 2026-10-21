package inspect

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/vshulcz/deja-vu/internal/model"
)

func TestSanitizeStringAndCap(t *testing.T) {
	// Secret key that must be redacted
	secretKey := "AKIAIOSFODNN7EXAMPLE"
	raw := "API Key is " + secretKey + " and here is a very long string that should exceed the limit: " + strings.Repeat("x", 400)

	sanitized := sanitizeString(raw, 50)
	if strings.Contains(sanitized, secretKey) {
		t.Errorf("secret AWS key was not redacted: %q", sanitized)
	}
	if !strings.HasSuffix(sanitized, "…") {
		t.Errorf("expected string to be capped with ellipsis: %q", sanitized)
	}
	if len([]rune(sanitized)) > 55 {
		t.Errorf("string length exceeds capped budget: %d runes", len([]rune(sanitized)))
	}
}

func TestPureTimelineProjectionEdgeCases(t *testing.T) {
	trace := &DeepSeekTrace{
		Log: "session.v3.jsonl",
		Header: DeepSeekHeader{
			Version:   3,
			ID:        "pure-001",
			CreatedAt: time.Now().UTC(),
		},
		Events: make([]DeepSeekEvent, 150),
	}

	for i := 0; i < 150; i++ {
		trace.Events[i] = DeepSeekEvent{
			Seq:     i,
			Type:    "step/start",
			Time:    time.Now().UTC(),
			RawHash: "hash" + strings.Repeat("0", 60),
			Data:    json.RawMessage(`{"turn":1,"step":` + strings.Repeat("1", 1) + `}`),
		}
	}

	// 1. Default limit (100) when limit <= 0
	view1, nextCursor1, comp1 := projectTimeline(trace, "", 0, "", "", "")
	if len(view1.Items) != 100 {
		t.Errorf("expected default limit 100, got %d", len(view1.Items))
	}
	if comp1 != CompletenessTruncated {
		t.Errorf("expected truncated completeness, got %q", comp1)
	}
	if nextCursor1 != "seq:100" {
		t.Errorf("nextCursor = %q, want seq:100", nextCursor1)
	}

	// 2. Cursor at 100 with limit 100
	view2, nextCursor2, comp2 := projectTimeline(trace, nextCursor1, 100, "", "", "")
	if len(view2.Items) != 50 {
		t.Errorf("expected 50 items remaining, got %d", len(view2.Items))
	}
	if comp2 != CompletenessComplete {
		t.Errorf("expected complete completeness, got %q", comp2)
	}
	if nextCursor2 != "" {
		t.Errorf("expected empty nextCursor, got %q", nextCursor2)
	}

	// 3. Cursor beyond max seq
	view3, nextCursor3, comp3 := projectTimeline(trace, "seq:999", 50, "", "", "")
	if len(view3.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(view3.Items))
	}
	if comp3 != CompletenessComplete {
		t.Errorf("expected complete, got %q", comp3)
	}
	if nextCursor3 != "" {
		t.Errorf("expected empty nextCursor, got %q", nextCursor3)
	}

	// 4. Hard cap 500 when limit > 500
	if clamped := clampLimit(1000); clamped != 500 {
		t.Errorf("clampLimit(1000) = %d, want 500", clamped)
	}
}

func TestPureEffectsCorrelationBySourceEventSeqs(t *testing.T) {
	trace := &DeepSeekTrace{
		Log: "session.v3.jsonl",
		Header: DeepSeekHeader{
			Version: 3,
			ID:      "effects-trace",
		},
		Events: []DeepSeekEvent{
			// tool/call without callId
			{
				Seq:     10,
				Type:    "tool/call",
				Time:    time.Now().UTC(),
				RawHash: "hash10",
				Data:    json.RawMessage(`{"name":"sandbox_exec","arguments":{"command":"git status"}}`),
			},
			// tool/result correlated by sourceEventSeqs [10]
			{
				Seq:             11,
				Type:            "tool/result",
				Time:            time.Now().UTC(),
				SourceEventSeqs: []int{10},
				RawHash:         "hash11",
				Data:            json.RawMessage(`{"content":"[sandbox: file access denied under default mode]"}`),
			},
		},
	}

	ef, _, comp := projectEffects(trace, "", 10)
	if comp != CompletenessComplete {
		t.Errorf("completeness = %q, want complete", comp)
	}
	if len(ef.Items) != 2 {
		t.Fatalf("expected 2 effect items, got %d", len(ef.Items))
	}

	req := ef.Items[0]
	if req.Stage != "requested" || req.Tool != "sandbox_exec" || req.Target != "git status" {
		t.Errorf("unexpected requested effect: %+v", req)
	}

	rep := ef.Items[1]
	if rep.Stage != "reported" || rep.Tool != "sandbox_exec" || rep.Outcome != "sandbox denial" {
		t.Errorf("unexpected reported effect: %+v", rep)
	}
}

func TestCleanEOFOpHavingOpenTurn(t *testing.T) {
	// A trace with clean EOF but open turn (no turn/end)
	trace := &DeepSeekTrace{
		Log: "session.v3.jsonl",
		Header: DeepSeekHeader{
			Version: 3,
			ID:      "open-turn-trace",
		},
		Truncated: false, // clean EOF
		Events: []DeepSeekEvent{
			{
				Seq:     0,
				Type:    "turn/start",
				Time:    time.Now().UTC(),
				RawHash: "hash0",
				Data:    json.RawMessage(`{"turn":0}`),
			},
			{
				Seq:     1,
				Type:    "step/start",
				Time:    time.Now().UTC(),
				RawHash: "hash1",
				Data:    json.RawMessage(`{"turn":0,"step":0}`),
			},
		},
	}

	sView := projectSession(trace, false, model.Session{}, "dummy.jsonl")
	if !sView.OpenTurn {
		t.Error("expected OpenTurn = true")
	}

	_, _, comp := projectTimeline(trace, "", 10, "", "", "")
	if comp != CompletenessComplete {
		t.Errorf("clean EOF must produce completeness complete even with open turn, got %q", comp)
	}
}
