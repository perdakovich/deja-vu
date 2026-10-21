package inspect

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"

	"strconv"
	"strings"

	"unicode/utf8"




	"github.com/vshulcz/deja-vu/internal/redact"
)

var (
	urlRE      = regexp.MustCompile(`https?://[^\s"'<>]+`)
	exitCodeRE = regexp.MustCompile(`\[exit code:\s*([0-9]+)\]`)
)

const (
	maxResourceCommands = 100
	maxResourceFiles    = 100
	maxResourceURLs     = 100
	maxResourceErrors   = 100
	maxLineageChildren  = 100
)

func parseTurnEndReason(data json.RawMessage) (kind string, detail string) {
	var s struct {
		Reason json.RawMessage `json:"reason"`
	}
	if err := json.Unmarshal(data, &s); err != nil || len(s.Reason) == 0 {
		return "completed", ""
	}
	var str string
	if err := json.Unmarshal(s.Reason, &str); err == nil {
		if str == "" {
			return "completed", ""
		}
		return str, ""
	}
	var obj struct {
		Kind    string `json:"kind"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(s.Reason, &obj); err == nil {
		k := obj.Kind
		if k == "" {
			k = "completed"
		}
		msg := obj.Message
		if msg == "" {
			msg = obj.Error
		}
		return k, msg
	}
	return "completed", ""
}

func isDisplayableTimelineEvent(ev DeepSeekEvent) bool {
	if ev.Type == "reasoning-chunks" {
		return false
	}
	if ev.Type == "user/message" {
		var d struct {
			Source struct {
				Kind string `json:"kind"`
			} `json:"source"`
		}
		if err := json.Unmarshal(ev.Data, &d); err == nil {
			if d.Source.Kind != "" && d.Source.Kind != "user" {
				return false
			}
		}
	}
	return true
}

func sanitizeString(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	redacted, _ := redact.Text(s)
	safe := strings.TrimSpace(redact.SafeForDisplay(redacted))
	if maxRunes > 0 && utf8.RuneCountInString(safe) > maxRunes {
		runes := []rune(safe)
		safe = string(runes[:maxRunes]) + "…"
	}
	return safe
}

func formatRef(log string, seq int, hash string) string {
	base := filepath.Base(log)
	if base == "" || base == "." {
		base = "session"
	}
	return fmt.Sprintf("%s#seq=%d#%s", base, seq, hash)
}

func parseSurfaceOp(raw json.RawMessage) (op string, startSeq, endSeq int) {
	if len(raw) == 0 {
		return "", 0, 0
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str, 0, 0
	}
	var detail struct {
		Op       string `json:"op"`
		StartSeq int    `json:"startSeq"`
		EndSeq   int    `json:"endSeq"`
	}
	if err := json.Unmarshal(raw, &detail); err == nil {
		return detail.Op, detail.StartSeq, detail.EndSeq
	}
	return "", 0, 0
}

func parseShadowedRange(raw json.RawMessage) *[2]int {
	if len(raw) == 0 {
		return nil
	}
	// Canonical DSH compaction shape uses {start: N, end: M}
	var obj struct {
		Start *int `json:"start"`
		End   *int `json:"end"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Start != nil && obj.End != nil {
		return &[2]int{*obj.Start, *obj.End}
	}
	// Alternate array shape [start, end]
	var arr [2]int
	if err := json.Unmarshal(raw, &arr); err == nil {
		return &arr
	}
	return nil
}

func parseCursor(cursor string) (int, bool) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, false
	}
	if strings.HasPrefix(cursor, "seq:") {
		n, err := strconv.Atoi(strings.TrimPrefix(cursor, "seq:"))
		if err == nil && n >= 0 {
			return n, true
		}
	}
	if strings.HasPrefix(cursor, "msg:") {
		n, err := strconv.Atoi(strings.TrimPrefix(cursor, "msg:"))
		if err == nil && n >= 0 {
			return n, true
		}
	}
	if n, err := strconv.Atoi(cursor); err == nil && n >= 0 {
		return n, true
	}
	return 0, false
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func findInheritedCut(events []DeepSeekEvent) int {
	inheritedCut := 0
	for _, ev := range events {
		if ev.Type == "session/end-seed" {
			var d struct {
				Inherited bool `json:"inherited"`
			}
			if err := json.Unmarshal(ev.Data, &d); err == nil && d.Inherited {
				inheritedCut = ev.Seq
			}
		}
	}
	return inheritedCut
}

